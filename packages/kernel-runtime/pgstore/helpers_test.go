package pgstore

import (
	"context"
	"io"
	"testing"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
	"github.com/jackc/pgx/v5/pgxpool"
)

type echoPackage struct{ manifest *contracts.Manifest }

func (p *echoPackage) Manifest() *contracts.Manifest { return p.manifest }
func (p *echoPackage) Initialize(context.Context, contracts.RuntimeContext) error {
	return nil
}
func (p *echoPackage) Shutdown(context.Context) error { return nil }
func (p *echoPackage) SupportedAuthMethods() []contracts.AuthMethod {
	return p.manifest.Spec.AuthMethods
}
func (p *echoPackage) StartAuth(context.Context, contracts.AuthConfig) (*contracts.AuthChallenge, error) {
	return nil, nil
}
func (p *echoPackage) CompleteAuth(context.Context, contracts.AuthResponse) (*contracts.CredentialRef, error) {
	return nil, nil
}
func (p *echoPackage) ValidateCredential(context.Context, *contracts.CredentialRef) error {
	return nil
}
func (p *echoPackage) RefreshCredential(context.Context, *contracts.CredentialRef) (*contracts.CredentialRef, error) {
	return nil, nil
}
func (p *echoPackage) RevokeCredential(context.Context, *contracts.CredentialRef) error { return nil }
func (p *echoPackage) ValidateConnection(context.Context, *contracts.Connection) (*contracts.ConnectionStatus, error) {
	return &contracts.ConnectionStatus{Healthy: true}, nil
}
func (p *echoPackage) Capabilities() []contracts.Capability { return p.manifest.Spec.Capabilities }
func (p *echoPackage) Execute(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
	return &contracts.OperationResult{Data: map[string]any{"echo": op.Input}}, nil
}

func newPGKernel(t *testing.T, pool *pgxpool.Pool) *kernel.Kernel {
	t.Helper()
	cfg := &config.Config{Execution: config.ExecutionConfig{MaxConcurrent: 2, Timeout: 5, MaxOutputBytes: 1 << 20}}
	k, err := kernel.New(cfg, logger.NewWithWriter(io.Discard, "error"),
		kernel.WithOperationStore(NewOperationStore(pool)), kernel.WithAuditStore(NewAuditStore(pool)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Shutdown(context.Background()) })
	err = k.RegisterPackage(&echoPackage{manifest: &contracts.Manifest{
		APIVersion: kernel.ManifestAPIVersion,
		Metadata:   contracts.ManifestMeta{Name: "echo", Version: "1.0.0", PackageAPIVersion: kernel.PackageAPIVersion},
		Spec: contracts.ManifestSpec{
			SupportedRuntimeVersions: []string{kernel.RuntimeVersion},
			ExecutionTarget:          "cloud",
			AuthMethods:              []contracts.AuthMethod{contracts.AuthAPIToken},
			Capabilities:             []contracts.Capability{{Name: "echo.read", Permissions: []string{"echo.read"}}},
			Operations:               []contracts.OperationDef{{ID: "echo", Capability: "echo.read", TimeoutSeconds: 5, RetryPolicy: "safe", Idempotent: true}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func submitEcho(k *kernel.Kernel) (*kernel.Operation, error) {
	return k.Submit(context.Background(), &contracts.OperationRequest{
		Integration: "echo", OperationID: "echo", Input: map[string]any{"msg": "hi"},
		Caller: &contracts.IdentityContext{UserID: "u1", Permissions: []string{"operation.execute", "echo.read"}},
	})
}
