package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/ids"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/permission"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

const maxIdempotencyKeyLen = 128

type requestIDKey struct{}

// WithRequestID attaches a request correlation ID to ctx
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the correlation ID attached to ctx, if any
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// runningOp tracks an accepted operation until it reaches a terminal state.
// mu serializes every state change and store write for this one operation,
// so a cancel request and the executor can never interleave writes, and no
// store I/O happens under the kernel-wide lock.
type runningOp struct {
	mu       sync.Mutex
	op       *Operation
	cancel   context.CancelFunc
	finished bool
}

// Submit runs the operation lifecycle: validate, authorize, resolve, execute,
// normalize, persist, and audit. A returned error is a *contracts.Error and
// means the request was rejected before any operation record was created.
// Once accepted, every outcome — including failures — is reported through
// the returned Operation's State and Error.
func (k *Kernel) Submit(ctx context.Context, req *contracts.OperationRequest) (*Operation, error) {
	if req == nil {
		return nil, contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_request", "operation request is required")
	}
	caller := req.Caller
	if caller == nil || (caller.UserID == "" && caller.AppID == "") {
		return nil, contracts.NewError(contracts.ErrorCategoryAuthFailed, "unauthenticated", "operation requires an authenticated caller")
	}
	if req.Integration == "" || req.OperationID == "" {
		return nil, contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_request", "integration and operationId are required")
	}
	if len(req.IdempotencyKey) > maxIdempotencyKeyLen {
		return nil, contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_idempotency_key", fmt.Sprintf("idempotency key must be at most %d characters", maxIdempotencyKeyLen))
	}

	k.mu.RLock()
	rp, ok := k.pkgs[req.Integration]
	k.mu.RUnlock()
	if !ok {
		return nil, contracts.NewError(contracts.ErrorCategoryNotFound, "integration_not_found", fmt.Sprintf("integration %q is not registered", req.Integration))
	}
	def, ok := rp.operations[req.OperationID]
	if !ok {
		return nil, contracts.NewError(contracts.ErrorCategoryNotFound, "operation_not_found", fmt.Sprintf("integration %q has no operation %q", req.Integration, req.OperationID))
	}
	capability := rp.capabilities[def.Capability]

	required := append([]string{string(permission.PermissionOperationExecute)}, capability.Permissions...)
	decision := k.permEngine.Authorize(granteeOf(caller), required)
	if !decision.Allowed {
		k.recordAudit(ctx, caller, "operation.authorize", req.Integration+"/"+req.OperationID, "denied", map[string]any{
			"reason":       decision.Reason,
			"connectionId": req.ConnectionID,
		})
		return nil, contracts.NewError(contracts.ErrorCategoryPermission, "permission_denied", decision.Reason)
	}

	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	if def.InputSchema != nil {
		if err := validateValue(def.InputSchema, input, "input"); err != nil {
			return nil, contracts.NewError(contracts.ErrorCategoryInvalidInput, "invalid_input", err.Error())
		}
	}

	// Phase 1 executes trusted packages in-process (ADR-002). Local-only
	// packages need the Local Agent, which does not exist yet.
	if rp.manifest.Spec.ExecutionTarget == "local" {
		return nil, contracts.NewError(contracts.ErrorCategoryNotSupported, "execution_target_unavailable", fmt.Sprintf("integration %q requires a Local Agent, which is not available", req.Integration))
	}

	if prior, err := k.priorOperation(ctx, caller, req); prior != nil || err != nil {
		return prior, err
	}

	timeout := time.Duration(def.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = time.Duration(k.cfg.Execution.Timeout) * time.Second
	}
	if req.Timeout > 0 && req.Timeout < timeout {
		timeout = req.Timeout
	}

	op := &Operation{
		ID:              ids.WithPrefix("op"),
		RequestID:       RequestID(ctx),
		Integration:     req.Integration,
		OperationID:     req.OperationID,
		Capability:      def.Capability,
		ConnectionID:    req.ConnectionID,
		CallerUserID:    caller.UserID,
		CallerAppID:     caller.AppID,
		ExecutionTarget: "cloud",
		Authorization:   decision.Reason,
		IdempotencyKey:  req.IdempotencyKey,
		Idempotent:      def.Idempotent,
		TimeoutSeconds:  int(timeout / time.Second),
		State:           contracts.StateQueued,
		CreatedAt:       time.Now().UTC(),
	}

	// The operation outlives the caller's request context: a dropped client
	// connection must not silently abort a write. Cancellation is explicit,
	// through CancelOperation, or via the operation timeout.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	entry := &runningOp{op: op, cancel: cancel}

	// Register before persisting so a cancel that races the insert still
	// finds the operation; deregister if the insert fails.
	k.mu.Lock()
	k.running[op.ID] = entry
	k.mu.Unlock()
	if err := k.ops.Create(ctx, op); err != nil {
		k.mu.Lock()
		delete(k.running, op.ID)
		k.mu.Unlock()
		if errors.Is(err, ErrDuplicateIdempotencyKey) {
			// A concurrent request with the same key won the insert
			if prior, perr := k.priorOperation(ctx, caller, req); prior != nil || perr != nil {
				return prior, perr
			}
		}
		k.log.Error("failed to persist operation", "request_id", op.RequestID, "error", err.Error())
		return nil, contracts.NewError(contracts.ErrorCategoryInternal, "persistence_failed", "failed to record operation")
	}

	execReq := *req
	execReq.ID = op.ID
	execReq.Input = input
	execReq.Timeout = timeout
	k.execute(runCtx, rp, def, entry, &execReq)
	return op, nil
}

// priorOperation returns the caller's earlier operation for req's idempotency
// key, nil if there is none, or a conflict if the key was used for a
// different request.
func (k *Kernel) priorOperation(ctx context.Context, caller *contracts.IdentityContext, req *contracts.OperationRequest) (*Operation, error) {
	if req.IdempotencyKey == "" {
		return nil, nil
	}
	prior, err := k.ops.FindByIdempotencyKey(ctx, caller.UserID, caller.AppID, req.IdempotencyKey)
	if errors.Is(err, ErrOperationNotFound) {
		return nil, nil
	}
	if err != nil {
		k.log.Error("idempotency lookup failed", "error", err.Error())
		return nil, contracts.NewError(contracts.ErrorCategoryInternal, "persistence_failed", "failed to check idempotency key")
	}
	if prior.Integration != req.Integration || prior.OperationID != req.OperationID || prior.ConnectionID != req.ConnectionID {
		return nil, contracts.NewError(contracts.ErrorCategoryConflict, "idempotency_key_reused", "idempotency key was already used for a different operation")
	}
	return prior, nil
}

type execOutcome struct {
	result   *contracts.OperationResult
	err      error
	panicked any
}

// execute waits for a concurrency slot, runs the package, and finalizes the operation
func (k *Kernel) execute(ctx context.Context, rp *registeredPackage, def contracts.OperationDef, entry *runningOp, req *contracts.OperationRequest) {
	op := entry.op
	log := k.log.With("operation_id", op.ID, "request_id", op.RequestID, "integration", op.Integration, "operation", op.OperationID)

	select {
	case k.slots <- struct{}{}:
	case <-ctx.Done():
		// Never started, so cancelling is safe regardless of idempotency
		k.finalize(ctx, entry, contracts.StateCancelled, contracts.NewError(contracts.ErrorCategoryInternal, "cancelled", "operation was cancelled before it started"), nil, log)
		return
	}

	started := time.Now().UTC()
	entry.mu.Lock()
	op.StartedAt = &started
	if op.State == contracts.StateQueued {
		op.State = contracts.StateRunning
	}
	k.persist(ctx, op, log)
	entry.mu.Unlock()
	log.Info("operation started")

	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	done := make(chan execOutcome, 1)
	go func() {
		// The slot is held until the package actually returns, so a package
		// that ignores cancellation still counts against the concurrency bound.
		defer func() { <-k.slots }()
		defer func() {
			if p := recover(); p != nil {
				done <- execOutcome{panicked: p}
			}
		}()
		res, err := rp.pkg.Execute(execCtx, req)
		done <- execOutcome{result: res, err: err}
	}()

	var out execOutcome
	select {
	case out = <-done:
	case <-execCtx.Done():
		out = execOutcome{err: execCtx.Err()}
	}

	state, nerr, result := k.classify(execCtx, def, out, log)
	k.finalize(ctx, entry, state, nerr, result, log)
	log.Info("operation finished", "state", op.State, "duration_ms", time.Since(started).Milliseconds())
}

// classify maps a package outcome onto the operation state model. When the
// kernel cannot know whether a non-idempotent operation took effect, the state
// is UnknownOutcome so nobody retries it blindly.
func (k *Kernel) classify(execCtx context.Context, def contracts.OperationDef, out execOutcome, log *logger.Logger) (contracts.OperationState, *contracts.Error, *contracts.OperationResult) {
	unknown := func(cause string) *contracts.Error {
		e := contracts.NewError(contracts.ErrorCategoryInternal, "outcome_unknown",
			cause+"; the operation may or may not have taken effect and must be reconciled before retrying")
		e.Retryable = false
		return e
	}

	if out.panicked != nil {
		log.Error("integration panicked", "panic", fmt.Sprint(out.panicked))
		if !def.Idempotent {
			return contracts.StateUnknownOutcome, unknown("integration crashed during execution"), nil
		}
		return contracts.StateFailed, contracts.NewError(contracts.ErrorCategoryInternal, "integration_crashed", "integration crashed during execution"), nil
	}

	ctxErr := execCtx.Err()
	if out.err != nil && ctxErr != nil && (errors.Is(out.err, context.DeadlineExceeded) || errors.Is(out.err, context.Canceled)) {
		switch {
		case errors.Is(ctxErr, context.DeadlineExceeded) && def.Idempotent:
			return contracts.StateTimedOut, contracts.NewError(contracts.ErrorCategoryTimeout, "timed_out", "operation exceeded its timeout"), nil
		case errors.Is(ctxErr, context.DeadlineExceeded):
			return contracts.StateUnknownOutcome, unknown("operation exceeded its timeout"), nil
		case def.Idempotent:
			return contracts.StateCancelled, contracts.NewError(contracts.ErrorCategoryInternal, "cancelled", "operation was cancelled"), nil
		default:
			return contracts.StateUnknownOutcome, unknown("operation was cancelled while running"), nil
		}
	}

	if out.err != nil {
		if nerr, ok := contracts.AsError(out.err); ok {
			return contracts.StateFailed, nerr, nil
		}
		// Raw errors may carry provider detail that is unsafe to return to
		// clients; log server-side, return a generic normalized error.
		log.Error("integration returned an unnormalized error", "error", out.err.Error())
		return contracts.StateFailed, contracts.NewError(contracts.ErrorCategoryInternal, "integration_error", "integration failed without a normalized error"), nil
	}
	if out.result == nil {
		return contracts.StateFailed, contracts.NewError(contracts.ErrorCategoryInternal, "empty_result", "integration returned no result"), nil
	}
	if out.result.Error != nil {
		return contracts.StateFailed, out.result.Error, nil
	}

	encoded, err := json.Marshal(struct {
		Data     map[string]any `json:"data"`
		Metadata map[string]any `json:"metadata"`
	}{out.result.Data, out.result.Metadata})
	if err != nil {
		log.Error("integration result is not serializable", "error", err.Error())
		return contracts.StateFailed, contracts.NewError(contracts.ErrorCategoryInternal, "invalid_result", "integration returned a result that cannot be serialized"), nil
	}
	if len(encoded) > k.cfg.Execution.MaxOutputBytes {
		return contracts.StateFailed, contracts.NewError(contracts.ErrorCategoryProvider, "output_too_large",
			fmt.Sprintf("result is %d bytes, exceeding the %d byte limit", len(encoded), k.cfg.Execution.MaxOutputBytes)), nil
	}
	return contracts.StateSucceeded, nil, out.result
}

// finalize persists the terminal state, deregisters the operation, and
// records the audit event
func (k *Kernel) finalize(ctx context.Context, entry *runningOp, state contracts.OperationState, nerr *contracts.Error, result *contracts.OperationResult, log *logger.Logger) {
	op := entry.op
	finished := time.Now().UTC()

	entry.mu.Lock()
	entry.finished = true
	op.State = state
	op.Error = nerr
	op.FinishedAt = &finished
	if result != nil {
		op.Result = result.Data
		op.ResultMetadata = result.Metadata
	}
	k.persist(ctx, op, log)
	entry.mu.Unlock()

	k.mu.Lock()
	delete(k.running, op.ID)
	k.mu.Unlock()

	meta := map[string]any{
		"operationId":  op.ID,
		"connectionId": op.ConnectionID,
		"requestId":    op.RequestID,
	}
	if nerr != nil {
		meta["errorCategory"] = string(nerr.Category)
		meta["errorCode"] = nerr.Code
	}
	k.recordAudit(ctx, &contracts.IdentityContext{UserID: op.CallerUserID, AppID: op.CallerAppID},
		"operation.execute", op.Integration+"/"+op.OperationID, string(state), meta)
}

// persist writes op to the store. A failed write is logged, not fatal: the
// in-flight execution is already committed to finishing.
func (k *Kernel) persist(ctx context.Context, op *Operation, log *logger.Logger) {
	if err := k.ops.Update(ctx, op); err != nil {
		log.Error("failed to persist operation state", "state", op.State, "error", err.Error())
	}
}

// GetOperation returns an operation visible to the caller. Operations owned
// by someone else are reported as not found rather than forbidden so IDs
// cannot be probed.
func (k *Kernel) GetOperation(ctx context.Context, caller *contracts.IdentityContext, id string) (*Operation, error) {
	op, err := k.ops.Get(ctx, id)
	if errors.Is(err, ErrOperationNotFound) || (err == nil && !k.canSee(caller, op)) {
		return nil, contracts.NewError(contracts.ErrorCategoryNotFound, "operation_not_found", "operation not found")
	}
	if err != nil {
		k.log.Error("operation lookup failed", "operation_id", id, "error", err.Error())
		return nil, contracts.NewError(contracts.ErrorCategoryInternal, "persistence_failed", "failed to load operation")
	}
	return op, nil
}

// ListOperations returns the caller's operations, newest first. Admins see all.
func (k *Kernel) ListOperations(ctx context.Context, caller *contracts.IdentityContext, limit int) ([]*Operation, error) {
	if caller == nil {
		return nil, contracts.NewError(contracts.ErrorCategoryAuthFailed, "unauthenticated", "caller is required")
	}
	f := OperationFilter{Limit: limit}
	if !k.permEngine.HasPermission(granteeOf(caller), permission.PermissionAdmin) {
		f.CallerUserID, f.CallerAppID = caller.UserID, caller.AppID
	}
	ops, err := k.ops.List(ctx, f)
	if err != nil {
		k.log.Error("operation list failed", "error", err.Error())
		return nil, contracts.NewError(contracts.ErrorCategoryInternal, "persistence_failed", "failed to list operations")
	}
	return ops, nil
}

// CancelOperation requests cancellation of a queued or running operation.
// The final state is set by the executor once the package stops.
func (k *Kernel) CancelOperation(ctx context.Context, caller *contracts.IdentityContext, id string) (*Operation, error) {
	op, err := k.GetOperation(ctx, caller, id)
	if err != nil {
		return nil, err
	}
	if !k.permEngine.HasPermission(granteeOf(caller), permission.PermissionOperationCancel) {
		return nil, contracts.NewError(contracts.ErrorCategoryPermission, "permission_denied", "missing required permission: operation.cancel")
	}

	k.mu.RLock()
	entry, running := k.running[id]
	k.mu.RUnlock()
	if !running {
		return nil, contracts.NewError(contracts.ErrorCategoryConflict, "operation_finished", fmt.Sprintf("operation is already %s", op.State))
	}

	entry.mu.Lock()
	if entry.finished {
		state := entry.op.State
		entry.mu.Unlock()
		return nil, contracts.NewError(contracts.ErrorCategoryConflict, "operation_finished", fmt.Sprintf("operation is already %s", state))
	}
	entry.op.State = contracts.StateCancelRequested
	k.persist(ctx, entry.op, k.log.With("operation_id", id))
	snapshot := *entry.op
	entry.cancel()
	entry.mu.Unlock()

	k.recordAudit(ctx, caller, "operation.cancel", op.Integration+"/"+op.OperationID, "requested", map[string]any{"operationId": id})
	return &snapshot, nil
}

func (k *Kernel) canSee(caller *contracts.IdentityContext, op *Operation) bool {
	if caller == nil {
		return false
	}
	if k.permEngine.HasPermission(granteeOf(caller), permission.PermissionAdmin) {
		return true
	}
	return op.CallerUserID == caller.UserID && op.CallerAppID == caller.AppID
}

func (k *Kernel) recordAudit(ctx context.Context, caller *contracts.IdentityContext, action, target, outcome string, meta map[string]any) {
	actor, actorType := "anonymous", "unknown"
	switch {
	case caller == nil:
	case caller.UserID != "":
		actor, actorType = caller.UserID, "user"
	case caller.AppID != "":
		actor, actorType = caller.AppID, "application"
	}
	if rid := RequestID(ctx); rid != "" {
		meta["requestId"] = rid
	}
	err := k.auditRec.Record(ctx, audit.Event{
		Actor:      actor,
		ActorType:  actorType,
		Action:     action,
		TargetType: "operation",
		TargetID:   target,
		Outcome:    outcome,
		Metadata:   meta,
	})
	if err != nil {
		// Audit loss is a security-relevant failure: log loudly with enough
		// context to reconstruct the event.
		k.log.Error("failed to record audit event", "action", action, "actor", actor, "target", target, "outcome", outcome, "error", err.Error())
	}
}

func granteeOf(c *contracts.IdentityContext) *permission.Grantee {
	if c == nil {
		return nil
	}
	return &permission.Grantee{UserID: c.UserID, AppID: c.AppID, DeviceID: c.DeviceID, Permissions: c.Permissions}
}
