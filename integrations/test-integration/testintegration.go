// Package testintegration is a dependency-free reference integration used to
// prove the kernel contract end to end. It talks to no external service.
// It is only registered when KERNEL_ENABLE_TEST_INTEGRATION=true.
package testintegration

import (
	"context"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// Name is the integration identifier
const Name = "test-integration"

// Integration implements contracts.Package
type Integration struct {
	manifest *contracts.Manifest
	runtime  contracts.RuntimeContext
}

// NewIntegration creates the test integration
func NewIntegration() *Integration {
	return &Integration{manifest: &contracts.Manifest{
		APIVersion: "omni/v1",
		Metadata: contracts.ManifestMeta{
			Name:              Name,
			DisplayName:       "Test Integration",
			Version:           "1.0.0",
			PackageAPIVersion: "v1",
			Description:       "Reference integration for kernel contract and API tests",
		},
		Spec: contracts.ManifestSpec{
			SupportedRuntimeVersions: []string{"v1.0"},
			ExecutionTarget:          "cloud",
			AuthMethods:              []contracts.AuthMethod{contracts.AuthAPIToken},
			Capabilities: []contracts.Capability{
				{Name: "test.read", Permissions: []string{"test.read"}, Description: "Read-only test operations"},
				{Name: "test.write", Permissions: []string{"test.write"}, Description: "Side-effecting test operations"},
			},
			Operations: []contracts.OperationDef{
				{
					ID: "echo", Capability: "test.read", TimeoutSeconds: 5, RetryPolicy: "safe", Idempotent: true,
					InputSchema: map[string]any{
						"type":                 "object",
						"required":             []any{"message"},
						"additionalProperties": false,
						"properties":           map[string]any{"message": map[string]any{"type": "string"}},
					},
				},
				{
					ID: "fail", Capability: "test.read", TimeoutSeconds: 5, RetryPolicy: "safe", Idempotent: true,
					InputSchema: map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"category": map[string]any{"type": "string", "enum": []any{"not_found", "rate_limited", "provider_error"}},
						},
					},
				},
				{
					ID: "sleep", Capability: "test.read", TimeoutSeconds: 30, RetryPolicy: "safe", Idempotent: true,
					InputSchema: map[string]any{
						"type":                 "object",
						"required":             []any{"milliseconds"},
						"additionalProperties": false,
						"properties":           map[string]any{"milliseconds": map[string]any{"type": "integer"}},
					},
				},
				{
					ID: "write", Capability: "test.write", TimeoutSeconds: 30, RetryPolicy: "unsafe", Idempotent: false,
					InputSchema: map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties":           map[string]any{"milliseconds": map[string]any{"type": "integer"}},
					},
				},
			},
		},
	}}
}

// Manifest returns the package manifest
func (i *Integration) Manifest() *contracts.Manifest { return i.manifest }

// Initialize stores the runtime context
func (i *Integration) Initialize(ctx context.Context, rt contracts.RuntimeContext) error {
	i.runtime = rt
	return nil
}

// Shutdown releases nothing; the integration holds no resources
func (i *Integration) Shutdown(ctx context.Context) error { return nil }

// SupportedAuthMethods returns the declared auth methods
func (i *Integration) SupportedAuthMethods() []contracts.AuthMethod {
	return i.manifest.Spec.AuthMethods
}

// StartAuth is not supported: the test integration needs no credentials
func (i *Integration) StartAuth(ctx context.Context, c contracts.AuthConfig) (*contracts.AuthChallenge, error) {
	return nil, notSupported()
}

// CompleteAuth is not supported
func (i *Integration) CompleteAuth(ctx context.Context, r contracts.AuthResponse) (*contracts.CredentialRef, error) {
	return nil, notSupported()
}

// ValidateCredential is not supported
func (i *Integration) ValidateCredential(ctx context.Context, c *contracts.CredentialRef) error {
	return notSupported()
}

// RefreshCredential is not supported
func (i *Integration) RefreshCredential(ctx context.Context, c *contracts.CredentialRef) (*contracts.CredentialRef, error) {
	return nil, notSupported()
}

// RevokeCredential is not supported
func (i *Integration) RevokeCredential(ctx context.Context, c *contracts.CredentialRef) error {
	return notSupported()
}

// ValidateConnection always reports healthy
func (i *Integration) ValidateConnection(ctx context.Context, conn *contracts.Connection) (*contracts.ConnectionStatus, error) {
	return &contracts.ConnectionStatus{Healthy: true, LastChecked: time.Now()}, nil
}

// Capabilities returns the declared capabilities
func (i *Integration) Capabilities() []contracts.Capability { return i.manifest.Spec.Capabilities }

// Execute runs one of the test operations
func (i *Integration) Execute(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
	switch op.OperationID {
	case "echo":
		return &contracts.OperationResult{Data: map[string]any{"message": op.Input["message"]}}, nil

	case "fail":
		cat, _ := op.Input["category"].(string)
		if cat == "" {
			cat = string(contracts.ErrorCategoryProvider)
		}
		return nil, contracts.NewError(contracts.ErrorCategory(cat), "test_failure", "requested failure")

	case "sleep", "write":
		ms, _ := op.Input["milliseconds"].(float64)
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if op.OperationID == "write" && i.runtime != nil {
			_ = i.runtime.EmitAuditEvent(ctx, contracts.AuditEvent{Action: "test.write", Target: "test-resource", Outcome: "written"})
		}
		return &contracts.OperationResult{Data: map[string]any{"sleptMs": ms}}, nil

	default:
		return nil, contracts.NewError(contracts.ErrorCategoryNotSupported, "unknown_operation", "operation is not implemented")
	}
}

func notSupported() error {
	return contracts.NewError(contracts.ErrorCategoryNotSupported, "auth_not_supported", "the test integration does not use credentials")
}
