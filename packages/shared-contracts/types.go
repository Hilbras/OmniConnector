package contracts

import (
	"time"
)

// IdentityContext identifies the caller of an operation
type IdentityContext struct {
	UserID      string
	AppID       string
	DeviceID    string
	Permissions []string
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Method AuthMethod
	Params map[string]any
}

// AuthChallenge represents a challenge in an auth flow
type AuthChallenge struct {
	Type string // "redirect", "prompt", "apiToken"
	URL  string
	TTL  time.Duration
}

// AuthResponse is the response to an auth challenge
type AuthResponse struct {
	Type   string
	Code   string
	Params map[string]any
}

// CredentialRef references a stored credential
type CredentialRef struct {
	ID       string
	Type     string
	Metadata map[string]any
}

// CredentialMaterial contains actual credential data
type CredentialMaterial struct {
	Data map[string]string
}

// Connection represents a configured service connection
type Connection struct {
	ID          string
	Integration string
	Config      map[string]any
	Credential  *CredentialRef
}

// ConnectionStatus represents the health status of a connection
type ConnectionStatus struct {
	Healthy     bool
	LastChecked time.Time
	Latency     time.Duration
	Error       *Error
	Metadata    map[string]any
}

// OperationRequest represents a request to execute an operation
type OperationRequest struct {
	ID             string
	ConnectionID   string
	Integration    string
	OperationID    string
	Input          map[string]any
	Caller         *IdentityContext
	IdempotencyKey string
	Timeout        time.Duration
	Metadata       map[string]any
}

// OperationResult represents the result of an operation
type OperationResult struct {
	Data     map[string]any `json:"data,omitempty"`
	Error    *Error         `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Audit    AuditMetadata  `json:"-"`
}

// AuditMetadata contains metadata for audit logging
type AuditMetadata struct {
	Actor       string
	Action      string
	Target      string
	Outcome     string
	Permissions []string
	Timestamp   time.Time
}

// ErrorCategory is a normalized error category
type ErrorCategory string

const (
	ErrorCategoryAuthFailed   ErrorCategory = "authentication_failed"
	ErrorCategoryAuthExpired  ErrorCategory = "authentication_expired"
	ErrorCategoryPermission   ErrorCategory = "permission_denied"
	ErrorCategoryNotFound     ErrorCategory = "not_found"
	ErrorCategoryRateLimited  ErrorCategory = "rate_limited"
	ErrorCategoryTimeout      ErrorCategory = "timeout"
	ErrorCategoryNetwork      ErrorCategory = "network_error"
	ErrorCategoryInvalidInput ErrorCategory = "invalid_input"
	ErrorCategoryProvider     ErrorCategory = "provider_error"
	ErrorCategoryInternal     ErrorCategory = "internal_error"
	ErrorCategoryNotSupported ErrorCategory = "not_supported"
	ErrorCategoryConflict     ErrorCategory = "conflict"
)

// Error represents a normalized error
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Category  ErrorCategory  `json:"category"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable"`
}

// OperationState represents the state of an operation
type OperationState string

const (
	StatePending          OperationState = "pending"
	StateValidating       OperationState = "validating"
	StateAwaitingApproval OperationState = "awaiting_approval"
	StateQueued           OperationState = "queued"
	StateRunning          OperationState = "running"
	StateSucceeded        OperationState = "succeeded"
	StateFailed           OperationState = "failed"
	StateCancelRequested  OperationState = "cancel_requested"
	StateCancelled        OperationState = "cancelled"
	StateTimedOut         OperationState = "timed_out"
	StateUnknownOutcome   OperationState = "unknown_outcome"
)

// OperationProgress represents progress of an operation
type OperationProgress struct {
	OperationID string
	State       OperationState
	Progress    float64
	Message     string
}
