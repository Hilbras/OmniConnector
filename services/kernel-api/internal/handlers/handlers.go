package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/permission"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
	"github.com/hilbras/omniconnector/services/kernel-api/internal/authn"
)

const (
	maxBodyBytes = 1 << 20
	defaultLimit = 50
	maxLimit     = 200
)

// Handler holds API handlers
type Handler struct {
	kernel *kernel.Kernel
	log    *logger.Logger
	authn  authn.Authenticator
}

// New creates a handler serving the given kernel
func New(k *kernel.Kernel, log *logger.Logger, a authn.Authenticator) *Handler {
	return &Handler{kernel: k, log: log, authn: a}
}

// Routes returns the complete API handler, middleware included
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health endpoints are unauthenticated
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /ready", h.Ready)

	// Integrations
	mux.HandleFunc("GET /v1/integrations", h.authenticated(h.ListIntegrations))
	mux.HandleFunc("GET /v1/integrations/{integrationId}", h.authenticated(h.GetIntegration))
	mux.HandleFunc("GET /v1/integrations/{integrationId}/capabilities", h.authenticated(h.ListCapabilities))

	// Connections and approvals arrive in Phase 2. Until then these routes
	// say so plainly instead of pretending to succeed.
	for _, route := range []string{
		"GET /v1/connections", "POST /v1/connections",
		"GET /v1/connections/{connectionId}", "PATCH /v1/connections/{connectionId}", "DELETE /v1/connections/{connectionId}",
		"POST /v1/connections/{connectionId}/test",
		"GET /v1/approvals", "POST /v1/approvals/{approvalId}/approve", "POST /v1/approvals/{approvalId}/reject",
	} {
		mux.HandleFunc(route, h.authenticated(notImplemented))
	}

	// Operations
	mux.HandleFunc("GET /v1/operations", h.authenticated(h.ListOperations))
	mux.HandleFunc("POST /v1/operations", h.authenticated(h.CreateOperation))
	mux.HandleFunc("GET /v1/operations/{operationId}", h.authenticated(h.GetOperation))
	mux.HandleFunc("POST /v1/operations/{operationId}/cancel", h.authenticated(h.CancelOperation))

	// Audit
	mux.HandleFunc("GET /v1/audit-events", h.authenticated(h.ListAuditEvents))

	return h.withObservability(mux)
}

// Health returns health status
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.kernel.Health())
}

// Ready reports whether the kernel and its stores can serve traffic
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.kernel.Ready(ctx); err != nil {
		h.log.Warn("readiness check failed", "error", err.Error())
		writeError(w, r, http.StatusServiceUnavailable, contracts.NewError(contracts.ErrorCategoryInternal, "not_ready", "a backing store is unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type integrationSummary struct {
	Name         string                   `json:"name"`
	DisplayName  string                   `json:"displayName"`
	Version      string                   `json:"version"`
	Description  string                   `json:"description,omitempty"`
	Capabilities []contracts.Capability   `json:"capabilities"`
	AuthMethods  []contracts.AuthMethod   `json:"authMethods,omitempty"`
	Operations   []contracts.OperationDef `json:"operations,omitempty"`
}

func summarize(m *contracts.Manifest, detailed bool) integrationSummary {
	s := integrationSummary{
		Name:         m.Metadata.Name,
		DisplayName:  m.Metadata.DisplayName,
		Version:      m.Metadata.Version,
		Description:  m.Metadata.Description,
		Capabilities: m.Spec.Capabilities,
	}
	if detailed {
		s.AuthMethods = m.Spec.AuthMethods
		s.Operations = m.Spec.Operations
	}
	return s
}

// ListIntegrations returns all registered integrations
func (h *Handler) ListIntegrations(w http.ResponseWriter, r *http.Request) {
	pkgs := h.kernel.ListPackages()
	result := make([]integrationSummary, 0, len(pkgs))
	for _, pkg := range pkgs {
		result = append(result, summarize(pkg.Manifest(), false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": result})
}

// GetIntegration returns a single integration
func (h *Handler) GetIntegration(w http.ResponseWriter, r *http.Request) {
	pkg, ok := h.kernel.GetPackage(r.PathValue("integrationId"))
	if !ok {
		writeError(w, r, http.StatusNotFound, contracts.NewError(contracts.ErrorCategoryNotFound, "integration_not_found", "integration not found"))
		return
	}
	writeJSON(w, http.StatusOK, summarize(pkg.Manifest(), true))
}

// ListCapabilities returns the capabilities an integration declares
func (h *Handler) ListCapabilities(w http.ResponseWriter, r *http.Request) {
	pkg, ok := h.kernel.GetPackage(r.PathValue("integrationId"))
	if !ok {
		writeError(w, r, http.StatusNotFound, contracts.NewError(contracts.ErrorCategoryNotFound, "integration_not_found", "integration not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"capabilities": pkg.Manifest().Spec.Capabilities})
}

type createOperationRequest struct {
	Integration    string         `json:"integration"`
	OperationID    string         `json:"operationId"`
	ConnectionID   string         `json:"connectionId"`
	Input          map[string]any `json:"input"`
	TimeoutSeconds int            `json:"timeoutSeconds"`
}

// CreateOperation validates and executes an operation. The response is the
// operation record: a failed operation is still a successfully created
// resource, so it returns 201 with state "failed" and a normalized error.
func (h *Handler) CreateOperation(w http.ResponseWriter, r *http.Request) {
	var body createOperationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if body.TimeoutSeconds < 0 {
		writeError(w, r, http.StatusBadRequest, contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_request", "timeoutSeconds must not be negative"))
		return
	}

	op, err := h.kernel.Submit(r.Context(), &contracts.OperationRequest{
		Integration:    body.Integration,
		OperationID:    body.OperationID,
		ConnectionID:   body.ConnectionID,
		Input:          body.Input,
		Caller:         identityFrom(r.Context()),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Timeout:        time.Duration(body.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		writeKernelError(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/operations/"+op.ID)
	writeJSON(w, http.StatusCreated, op)
}

// ListOperations returns the caller's operation history
func (h *Handler) ListOperations(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	ops, kerr := h.kernel.ListOperations(r.Context(), identityFrom(r.Context()), limit)
	if kerr != nil {
		writeKernelError(w, r, kerr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operations": ops})
}

// GetOperation returns one operation
func (h *Handler) GetOperation(w http.ResponseWriter, r *http.Request) {
	op, err := h.kernel.GetOperation(r.Context(), identityFrom(r.Context()), r.PathValue("operationId"))
	if err != nil {
		writeKernelError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, op)
}

// CancelOperation requests cancellation; 202 because the stop is asynchronous
func (h *Handler) CancelOperation(w http.ResponseWriter, r *http.Request) {
	op, err := h.kernel.CancelOperation(r.Context(), identityFrom(r.Context()), r.PathValue("operationId"))
	if err != nil {
		writeKernelError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, op)
}

// ListAuditEvents returns audit events. Admins see all; others with
// audit.read see only events where they are the actor.
func (h *Handler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	caller := identityFrom(r.Context())
	grantee := &permission.Grantee{UserID: caller.UserID, AppID: caller.AppID, Permissions: caller.Permissions}
	engine := h.kernel.PermissionEngine()
	if !engine.HasPermission(grantee, permission.PermissionAuditRead) {
		writeError(w, r, http.StatusForbidden, contracts.NewError(contracts.ErrorCategoryPermission, "permission_denied", "missing required permission: audit.read"))
		return
	}
	limit, err := parseLimit(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}

	filter := audit.Filter{Limit: limit}
	if !engine.HasPermission(grantee, permission.PermissionAdmin) {
		filter.Actor = caller.UserID
		if filter.Actor == "" {
			filter.Actor = caller.AppID
		}
	}
	events, aerr := h.kernel.AuditRecorder().List(r.Context(), filter)
	if aerr != nil {
		h.log.Error("audit list failed", "error", aerr.Error())
		writeError(w, r, http.StatusInternalServerError, contracts.NewError(contracts.ErrorCategoryInternal, "persistence_failed", "failed to list audit events"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotImplemented, contracts.NewError(contracts.ErrorCategoryNotSupported, "not_implemented", "this endpoint is not implemented yet"))
}

// decodeJSON reads a bounded, strictly-typed JSON body
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) *contracts.Error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return contracts.NewError(contracts.ErrorCategoryInvalidInput, "unsupported_media_type", "request body must be application/json")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return contracts.NewError(contracts.ErrorCategoryInvalidInput, "body_too_large", "request body exceeds 1 MiB")
		}
		return contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_json", "request body is not valid JSON for this endpoint: "+err.Error())
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_json", "request body must contain a single JSON object")
	}
	return nil
}

func parseLimit(r *http.Request) (int, *contracts.Error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxLimit {
		return 0, contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_limit", "limit must be an integer between 1 and 200")
	}
	return n, nil
}

// statusFor maps a normalized error category to an HTTP status
func statusFor(cat contracts.ErrorCategory) int {
	switch cat {
	case contracts.ErrorCategoryInvalidInput:
		return http.StatusBadRequest
	case contracts.ErrorCategoryAuthFailed, contracts.ErrorCategoryAuthExpired:
		return http.StatusUnauthorized
	case contracts.ErrorCategoryPermission:
		return http.StatusForbidden
	case contracts.ErrorCategoryNotFound:
		return http.StatusNotFound
	case contracts.ErrorCategoryConflict:
		return http.StatusConflict
	case contracts.ErrorCategoryRateLimited:
		return http.StatusTooManyRequests
	case contracts.ErrorCategoryNotSupported:
		return http.StatusNotImplemented
	case contracts.ErrorCategoryTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func writeKernelError(w http.ResponseWriter, r *http.Request, err error) {
	nerr, ok := contracts.AsError(err)
	if !ok {
		nerr = contracts.NewError(contracts.ErrorCategoryInternal, "internal_error", "internal server error")
	}
	writeError(w, r, statusFor(nerr.Category), nerr)
}

type errorEnvelope struct {
	Error struct {
		*contracts.Error
		RequestID string `json:"requestId,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, nerr *contracts.Error) {
	var env errorEnvelope
	env.Error.Error = nerr
	env.Error.RequestID = kernel.RequestID(r.Context())
	writeJSON(w, status, env)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
