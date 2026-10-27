package contracts

import (
	"context"
	"time"
)

// Authenticator manages authentication lifecycle
type Authenticator interface {
	SupportedAuthMethods() []AuthMethod
	StartAuth(ctx context.Context, config AuthConfig) (*AuthChallenge, error)
	CompleteAuth(ctx context.Context, response AuthResponse) (*CredentialRef, error)
	ValidateCredential(ctx context.Context, cred *CredentialRef) error
	RefreshCredential(ctx context.Context, cred *CredentialRef) (*CredentialRef, error)
	RevokeCredential(ctx context.Context, cred *CredentialRef) error
}

// ConnectionValidator tests whether a connection is usable
type ConnectionValidator interface {
	ValidateConnection(ctx context.Context, conn *Connection) (*ConnectionStatus, error)
}

// OperationExecutor executes operations
type OperationExecutor interface {
	Capabilities() []Capability
	Execute(ctx context.Context, op *OperationRequest) (*OperationResult, error)
}

// Package is the full integration package interface
type Package interface {
	Authenticator
	ConnectionValidator
	OperationExecutor

	Manifest() *Manifest
	Initialize(ctx context.Context, runtime RuntimeContext) error
	Shutdown(ctx context.Context) error
}

// RuntimeContext provides the integration access to kernel services
type RuntimeContext interface {
	Logger() Logger
	GetCredential(ctx context.Context, ref CredentialRef) (*CredentialMaterial, error)
	EmitAuditEvent(ctx context.Context, event AuditEvent) error
	ProgressChan() chan OperationProgress
}

// Logger is the minimal logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// AuditEvent represents an audit event
type AuditEvent struct {
	Actor       string
	Action      string
	Target      string
	Outcome     string
	Permissions []string
	Metadata    map[string]any
	Timestamp   time.Time
}
