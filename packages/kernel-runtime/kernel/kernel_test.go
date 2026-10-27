package kernel

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// fakePackage is a contracts.Package whose Execute behavior is set per test
type fakePackage struct {
	manifest    *contracts.Manifest
	execute     func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error)
	initErr     error
	initialized atomic.Bool
	shutdown    atomic.Bool
	calls       atomic.Int32
}

func (p *fakePackage) Manifest() *contracts.Manifest { return p.manifest }
func (p *fakePackage) Initialize(ctx context.Context, rt contracts.RuntimeContext) error {
	if p.initErr != nil {
		return p.initErr
	}
	p.initialized.Store(true)
	return nil
}
func (p *fakePackage) Shutdown(ctx context.Context) error { p.shutdown.Store(true); return nil }
func (p *fakePackage) SupportedAuthMethods() []contracts.AuthMethod {
	return p.manifest.Spec.AuthMethods
}
func (p *fakePackage) StartAuth(ctx context.Context, c contracts.AuthConfig) (*contracts.AuthChallenge, error) {
	return nil, nil
}
func (p *fakePackage) CompleteAuth(ctx context.Context, r contracts.AuthResponse) (*contracts.CredentialRef, error) {
	return nil, nil
}
func (p *fakePackage) ValidateCredential(ctx context.Context, c *contracts.CredentialRef) error {
	return nil
}
func (p *fakePackage) RefreshCredential(ctx context.Context, c *contracts.CredentialRef) (*contracts.CredentialRef, error) {
	return nil, nil
}
func (p *fakePackage) RevokeCredential(ctx context.Context, c *contracts.CredentialRef) error {
	return nil
}
func (p *fakePackage) ValidateConnection(ctx context.Context, c *contracts.Connection) (*contracts.ConnectionStatus, error) {
	return &contracts.ConnectionStatus{Healthy: true}, nil
}
func (p *fakePackage) Capabilities() []contracts.Capability { return p.manifest.Spec.Capabilities }
func (p *fakePackage) Execute(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
	p.calls.Add(1)
	if p.execute != nil {
		return p.execute(ctx, op)
	}
	return &contracts.OperationResult{Data: map[string]any{"echo": op.Input}}, nil
}

func validManifest(name string) *contracts.Manifest {
	return &contracts.Manifest{
		APIVersion: ManifestAPIVersion,
		Metadata: contracts.ManifestMeta{
			Name:              name,
			DisplayName:       "Test",
			Version:           "1.0.0",
			PackageAPIVersion: PackageAPIVersion,
		},
		Spec: contracts.ManifestSpec{
			SupportedRuntimeVersions: []string{RuntimeVersion},
			ExecutionTarget:          "cloud",
			AuthMethods:              []contracts.AuthMethod{contracts.AuthAPIToken},
			Capabilities: []contracts.Capability{
				{Name: "item.read", Permissions: []string{"item.read"}},
				{Name: "item.write", Permissions: []string{"item.write"}},
			},
			Operations: []contracts.OperationDef{
				{ID: "item.get", Capability: "item.read", TimeoutSeconds: 5, RetryPolicy: "safe", Idempotent: true,
					InputSchema: map[string]any{
						"type":                 "object",
						"required":             []any{"id"},
						"additionalProperties": false,
						"properties":           map[string]any{"id": map[string]any{"type": "string"}},
					}},
				{ID: "item.create", Capability: "item.write", TimeoutSeconds: 5, RetryPolicy: "unsafe", Idempotent: false},
			},
		},
	}
}

func newTestKernel(t *testing.T) *Kernel {
	t.Helper()
	cfg := &config.Config{
		HTTP:      config.HTTPConfig{Addr: ":0"},
		Log:       config.LogConfig{Level: "error"},
		Execution: config.ExecutionConfig{MaxConcurrent: 4, Timeout: 30, MaxOutputBytes: 1 << 16},
	}
	k, err := New(cfg, logger.NewWithWriter(io.Discard, "error"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = k.Shutdown(context.Background()) })
	return k
}

func register(t *testing.T, k *Kernel, p *fakePackage) {
	t.Helper()
	if err := k.RegisterPackage(p); err != nil {
		t.Fatalf("RegisterPackage: %v", err)
	}
}

var (
	reader = &contracts.IdentityContext{UserID: "u-reader", Permissions: []string{"operation.execute", "operation.cancel", "item.read"}}
	writer = &contracts.IdentityContext{UserID: "u-writer", Permissions: []string{"operation.execute", "operation.cancel", "item.*"}}
	admin  = &contracts.IdentityContext{UserID: "u-admin", Permissions: []string{"admin"}}
)

func submit(k *Kernel, caller *contracts.IdentityContext, opID string, input map[string]any) (*Operation, error) {
	return k.Submit(context.Background(), &contracts.OperationRequest{
		Integration: "test", OperationID: opID, Input: input, Caller: caller,
	})
}

func wantCategory(t *testing.T, err error, cat contracts.ErrorCategory) {
	t.Helper()
	nerr, ok := contracts.AsError(err)
	if !ok {
		t.Fatalf("expected normalized error %s, got %v", cat, err)
	}
	if nerr.Category != cat {
		t.Fatalf("expected category %s, got %s (%s)", cat, nerr.Category, nerr.Message)
	}
}

func TestRegisterPackage(t *testing.T) {
	k := newTestKernel(t)
	p := &fakePackage{manifest: validManifest("test")}
	register(t, k, p)

	if !p.initialized.Load() {
		t.Error("package was not initialized on registration")
	}
	if err := k.RegisterPackage(&fakePackage{manifest: validManifest("test")}); err == nil {
		t.Error("expected error on duplicate registration")
	}
	if _, ok := k.GetPackage("test"); !ok {
		t.Fatal("package not found after registration")
	}
	if n := len(k.ListPackages()); n != 1 {
		t.Errorf("expected 1 package, got %d", n)
	}
	if n := len(k.GetCapabilities()["test"]); n != 2 {
		t.Errorf("expected 2 capabilities, got %d", n)
	}
}

func TestRegisterPackageInitFailure(t *testing.T) {
	k := newTestKernel(t)
	if err := k.RegisterPackage(&fakePackage{manifest: validManifest("test"), initErr: errors.New("boom")}); err == nil {
		t.Fatal("expected registration to fail when Initialize fails")
	}
	if _, ok := k.GetPackage("test"); ok {
		t.Error("package that failed to initialize must not be registered")
	}
}

func TestShutdownCallsPackages(t *testing.T) {
	k := newTestKernel(t)
	p := &fakePackage{manifest: validManifest("test")}
	register(t, k, p)
	if err := k.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if !p.shutdown.Load() {
		t.Error("package Shutdown was not called")
	}
}

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *contracts.Manifest)
		wantErr string
	}{
		{"valid", func(m *contracts.Manifest) {}, ""},
		{"wrong apiVersion", func(m *contracts.Manifest) { m.APIVersion = "" }, "apiVersion"},
		{"bad name", func(m *contracts.Manifest) { m.Metadata.Name = "Bad Name" }, "metadata.name"},
		{"non-semver", func(m *contracts.Manifest) { m.Metadata.Version = "1" }, "metadata.version"},
		{"unsupported package api", func(m *contracts.Manifest) { m.Metadata.PackageAPIVersion = "v9" }, "packageApiVersion"},
		{"incompatible runtime", func(m *contracts.Manifest) { m.Spec.SupportedRuntimeVersions = []string{"v2.0"} }, "supportedRuntimeVersions"},
		{"bad execution target", func(m *contracts.Manifest) { m.Spec.ExecutionTarget = "mars" }, "executionTarget"},
		{"no auth methods", func(m *contracts.Manifest) { m.Spec.AuthMethods = nil }, "authMethods"},
		{"no capabilities", func(m *contracts.Manifest) { m.Spec.Capabilities = nil }, "capabilities must not be empty"},
		{"duplicate capability", func(m *contracts.Manifest) {
			m.Spec.Capabilities = append(m.Spec.Capabilities, m.Spec.Capabilities[0])
		}, "duplicate name"},
		{"wildcard requirement", func(m *contracts.Manifest) { m.Spec.Capabilities[0].Permissions = []string{"item.*"} }, "invalid required permission"},
		{"admin requirement", func(m *contracts.Manifest) { m.Spec.Capabilities[0].Permissions = []string{"admin"} }, "invalid required permission"},
		{"undeclared capability", func(m *contracts.Manifest) { m.Spec.Operations[0].Capability = "nope" }, "not declared"},
		{"duplicate operation", func(m *contracts.Manifest) {
			m.Spec.Operations = append(m.Spec.Operations, m.Spec.Operations[0])
		}, "duplicate id"},
		{"bad retry policy", func(m *contracts.Manifest) { m.Spec.Operations[0].RetryPolicy = "always" }, "retryPolicy"},
		{"safe but not idempotent", func(m *contracts.Manifest) { m.Spec.Operations[0].Idempotent = false }, "requires idempotent"},
		{"unsupported schema keyword", func(m *contracts.Manifest) {
			m.Spec.Operations[0].InputSchema = map[string]any{"type": "object", "pattern": "x"}
		}, "unsupported keyword"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest("test")
			tt.mutate(m)
			err := ValidateManifest(m)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid manifest, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSubmitSucceeds(t *testing.T) {
	k := newTestKernel(t)
	register(t, k, &fakePackage{manifest: validManifest("test")})

	op, err := submit(k, reader, "item.get", map[string]any{"id": "42"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if op.State != contracts.StateSucceeded {
		t.Fatalf("expected succeeded, got %s (%v)", op.State, op.Error)
	}
	echo, _ := op.Result["echo"].(map[string]any)
	if echo["id"] != "42" {
		t.Errorf("unexpected result: %v", op.Result)
	}
	if op.StartedAt == nil || op.FinishedAt == nil {
		t.Error("expected start and finish timestamps")
	}

	stored, err := k.GetOperation(context.Background(), reader, op.ID)
	if err != nil || stored.State != contracts.StateSucceeded {
		t.Fatalf("stored operation not retrievable: %v %v", stored, err)
	}

	events, _ := k.AuditRecorder().List(context.Background(), audit.Filter{})
	last := events[0]
	if last.Action != "operation.execute" || last.Outcome != "succeeded" || last.Actor != "u-reader" {
		t.Errorf("unexpected audit event: %+v", last)
	}
}

func TestSubmitRejections(t *testing.T) {
	k := newTestKernel(t)
	p := &fakePackage{manifest: validManifest("test")}
	register(t, k, p)

	_, err := submit(k, nil, "item.get", map[string]any{"id": "1"})
	wantCategory(t, err, contracts.ErrorCategoryAuthFailed)

	_, err = k.Submit(context.Background(), &contracts.OperationRequest{Integration: "missing", OperationID: "x", Caller: reader})
	wantCategory(t, err, contracts.ErrorCategoryNotFound)

	_, err = submit(k, reader, "item.delete", nil)
	wantCategory(t, err, contracts.ErrorCategoryNotFound)

	_, err = submit(k, reader, "item.create", nil)
	wantCategory(t, err, contracts.ErrorCategoryPermission)

	noExec := &contracts.IdentityContext{UserID: "u", Permissions: []string{"item.read"}}
	_, err = submit(k, noExec, "item.get", map[string]any{"id": "1"})
	wantCategory(t, err, contracts.ErrorCategoryPermission)

	_, err = submit(k, reader, "item.get", map[string]any{})
	wantCategory(t, err, contracts.ErrorCategoryInvalidInput)
	_, err = submit(k, reader, "item.get", map[string]any{"id": 7})
	wantCategory(t, err, contracts.ErrorCategoryInvalidInput)
	_, err = submit(k, reader, "item.get", map[string]any{"id": "1", "extra": true})
	wantCategory(t, err, contracts.ErrorCategoryInvalidInput)

	if p.calls.Load() != 0 {
		t.Errorf("rejected requests must never reach the package, got %d calls", p.calls.Load())
	}
	denied := 0
	events, _ := k.AuditRecorder().List(context.Background(), audit.Filter{})
	for _, e := range events {
		if e.Action == "operation.authorize" && e.Outcome == "denied" {
			denied++
		}
	}
	if denied != 2 {
		t.Errorf("expected 2 denied authorization audit events, got %d", denied)
	}
}

func TestSubmitNormalizesFailures(t *testing.T) {
	tests := []struct {
		name      string
		opID      string
		execute   func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error)
		wantState contracts.OperationState
		wantCode  string
		wantCat   contracts.ErrorCategory
	}{
		{
			name: "normalized error returned", opID: "item.get",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				return nil, contracts.NewError(contracts.ErrorCategoryNotFound, "item_missing", "no such item")
			},
			wantState: contracts.StateFailed, wantCode: "item_missing", wantCat: contracts.ErrorCategoryNotFound,
		},
		{
			name: "error in result", opID: "item.get",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				return &contracts.OperationResult{Error: contracts.NewError(contracts.ErrorCategoryRateLimited, "slow_down", "rate limited")}, nil
			},
			wantState: contracts.StateFailed, wantCode: "slow_down", wantCat: contracts.ErrorCategoryRateLimited,
		},
		{
			name: "raw error is not leaked", opID: "item.get",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				return nil, errors.New("dial tcp: password=hunter2")
			},
			wantState: contracts.StateFailed, wantCode: "integration_error", wantCat: contracts.ErrorCategoryInternal,
		},
		{
			name: "nil result", opID: "item.get",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				return nil, nil
			},
			wantState: contracts.StateFailed, wantCode: "empty_result", wantCat: contracts.ErrorCategoryInternal,
		},
		{
			name: "oversized output", opID: "item.get",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				return &contracts.OperationResult{Data: map[string]any{"blob": strings.Repeat("x", 1<<17)}}, nil
			},
			wantState: contracts.StateFailed, wantCode: "output_too_large", wantCat: contracts.ErrorCategoryProvider,
		},
		{
			name: "panic in idempotent op", opID: "item.get",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				panic("kaboom")
			},
			wantState: contracts.StateFailed, wantCode: "integration_crashed", wantCat: contracts.ErrorCategoryInternal,
		},
		{
			name: "panic in write op", opID: "item.create",
			execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				panic("kaboom")
			},
			wantState: contracts.StateUnknownOutcome, wantCode: "outcome_unknown", wantCat: contracts.ErrorCategoryInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := newTestKernel(t)
			register(t, k, &fakePackage{manifest: validManifest("test"), execute: tt.execute})
			var input map[string]any
			if tt.opID == "item.get" {
				input = map[string]any{"id": "1"}
			}
			op, err := submit(k, writer, tt.opID, input)
			if err != nil {
				t.Fatalf("Submit rejected: %v", err)
			}
			if op.State != tt.wantState {
				t.Fatalf("expected state %s, got %s", tt.wantState, op.State)
			}
			if op.Error == nil || op.Error.Code != tt.wantCode || op.Error.Category != tt.wantCat {
				t.Fatalf("expected %s/%s, got %+v", tt.wantCat, tt.wantCode, op.Error)
			}
			if strings.Contains(op.Error.Message, "hunter2") {
				t.Fatal("raw integration error leaked into operation record")
			}
			if op.Result != nil {
				t.Error("failed operation must not carry a result")
			}
		})
	}
}

func blockUntilDone(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestTimeouts(t *testing.T) {
	k := newTestKernel(t)
	register(t, k, &fakePackage{manifest: validManifest("test"), execute: blockUntilDone})

	read, err := k.Submit(context.Background(), &contracts.OperationRequest{
		Integration: "test", OperationID: "item.get", Input: map[string]any{"id": "1"}, Caller: writer, Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if read.State != contracts.StateTimedOut || !read.Error.Retryable {
		t.Errorf("idempotent timeout: expected retryable timed_out, got %s %+v", read.State, read.Error)
	}

	write, err := k.Submit(context.Background(), &contracts.OperationRequest{
		Integration: "test", OperationID: "item.create", Caller: writer, Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if write.State != contracts.StateUnknownOutcome || write.Error.Retryable {
		t.Errorf("write timeout: expected non-retryable unknown_outcome, got %s %+v", write.State, write.Error)
	}
}

func TestTimeoutWhenPackageIgnoresContext(t *testing.T) {
	k := newTestKernel(t)
	release := make(chan struct{})
	defer close(release)
	register(t, k, &fakePackage{manifest: validManifest("test"), execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
		<-release
		return &contracts.OperationResult{}, nil
	}})
	op, err := k.Submit(context.Background(), &contracts.OperationRequest{
		Integration: "test", OperationID: "item.get", Input: map[string]any{"id": "1"}, Caller: reader, Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if op.State != contracts.StateTimedOut {
		t.Fatalf("expected timed_out even though the package ignored ctx, got %s", op.State)
	}
}

func TestCancel(t *testing.T) {
	for _, tt := range []struct {
		opID  string
		input map[string]any
		want  contracts.OperationState
	}{
		{"item.get", map[string]any{"id": "1"}, contracts.StateCancelled},
		{"item.create", nil, contracts.StateUnknownOutcome},
	} {
		t.Run(tt.opID, func(t *testing.T) {
			k := newTestKernel(t)
			started := make(chan string, 1)
			register(t, k, &fakePackage{manifest: validManifest("test"), execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
				started <- op.ID
				return blockUntilDone(ctx, op)
			}})

			result := make(chan *Operation, 1)
			go func() {
				op, _ := submit(k, writer, tt.opID, tt.input)
				result <- op
			}()
			id := <-started

			if _, err := k.CancelOperation(context.Background(), reader, id); err == nil {
				t.Fatal("another user must not be able to cancel the operation")
			}
			op, err := k.CancelOperation(context.Background(), writer, id)
			if err != nil {
				t.Fatalf("CancelOperation: %v", err)
			}
			if op.State != contracts.StateCancelRequested {
				t.Errorf("expected cancel_requested, got %s", op.State)
			}

			final := <-result
			if final.State != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, final.State)
			}
			_, err = k.CancelOperation(context.Background(), writer, id)
			wantCategory(t, err, contracts.ErrorCategoryConflict)
		})
	}
}

func TestIdempotencyKey(t *testing.T) {
	k := newTestKernel(t)
	p := &fakePackage{manifest: validManifest("test")}
	register(t, k, p)

	req := func(op string) *contracts.OperationRequest {
		return &contracts.OperationRequest{Integration: "test", OperationID: op, Caller: writer, IdempotencyKey: "key-1"}
	}
	first, err := k.Submit(context.Background(), req("item.create"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := k.Submit(context.Background(), req("item.create"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || p.calls.Load() != 1 {
		t.Fatalf("retry with same key must return the original operation without re-executing (ids %s/%s, calls %d)", first.ID, second.ID, p.calls.Load())
	}

	r := req("item.get")
	r.Input = map[string]any{"id": "1"}
	_, err = k.Submit(context.Background(), r)
	wantCategory(t, err, contracts.ErrorCategoryConflict)

	// Keys are scoped per caller
	other := &contracts.IdentityContext{UserID: "u-other", Permissions: writer.Permissions}
	third, err := k.Submit(context.Background(), &contracts.OperationRequest{Integration: "test", OperationID: "item.create", Caller: other, IdempotencyKey: "key-1"})
	if err != nil || third.ID == first.ID {
		t.Fatalf("idempotency keys must not collide across callers: %v", err)
	}
}

func TestOperationVisibility(t *testing.T) {
	k := newTestKernel(t)
	register(t, k, &fakePackage{manifest: validManifest("test")})
	op, err := submit(k, writer, "item.create", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = k.GetOperation(context.Background(), reader, op.ID)
	wantCategory(t, err, contracts.ErrorCategoryNotFound)
	if _, err := k.GetOperation(context.Background(), admin, op.ID); err != nil {
		t.Errorf("admin should see every operation: %v", err)
	}
	if n := len(mustList(t, k, reader)); n != 0 {
		t.Errorf("reader should see no operations, got %d", n)
	}
	if n := len(mustList(t, k, writer)); n != 1 {
		t.Errorf("writer should see 1 operation, got %d", n)
	}
}

func TestLocalExecutionTargetRejected(t *testing.T) {
	k := newTestKernel(t)
	m := validManifest("test")
	m.Spec.ExecutionTarget = "local"
	p := &fakePackage{manifest: m}
	register(t, k, p)
	_, err := submit(k, reader, "item.get", map[string]any{"id": "1"})
	wantCategory(t, err, contracts.ErrorCategoryNotSupported)
	if p.calls.Load() != 0 {
		t.Error("local-only package must not execute in the cloud runtime")
	}
}

func TestConcurrencyBound(t *testing.T) {
	k := newTestKernel(t)
	var inFlight, peak atomic.Int32
	register(t, k, &fakePackage{manifest: validManifest("test"), execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		inFlight.Add(-1)
		return &contracts.OperationResult{}, nil
	}})

	done := make(chan struct{})
	for i := 0; i < 12; i++ {
		go func() {
			_, _ = submit(k, reader, "item.get", map[string]any{"id": "1"})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 12; i++ {
		<-done
	}
	if peak.Load() > 4 {
		t.Fatalf("concurrency bound of 4 exceeded: peak %d", peak.Load())
	}
}

func TestHealth(t *testing.T) {
	k := newTestKernel(t)
	h := k.Health()
	if h["status"] != "healthy" || h["version"] != Version {
		t.Errorf("unexpected health: %v", h)
	}
}

func mustList(t *testing.T, k *Kernel, caller *contracts.IdentityContext) []*Operation {
	t.Helper()
	ops, err := k.ListOperations(context.Background(), caller, 0)
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

// TestConcurrentIdempotentRetries fires the same keyed request concurrently:
// exactly one execution may happen and every caller sees the same operation.
func TestConcurrentIdempotentRetries(t *testing.T) {
	k := newTestKernel(t)
	release := make(chan struct{})
	p := &fakePackage{manifest: validManifest("test"), execute: func(ctx context.Context, op *contracts.OperationRequest) (*contracts.OperationResult, error) {
		<-release
		return &contracts.OperationResult{}, nil
	}}
	register(t, k, p)

	const n = 8
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			op, err := k.Submit(context.Background(), &contracts.OperationRequest{Integration: "test", OperationID: "item.create", Caller: writer, IdempotencyKey: "race"})
			if err != nil {
				ids <- "error: " + err.Error()
				return
			}
			ids <- op.ID
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)

	first := <-ids
	for i := 1; i < n; i++ {
		if got := <-ids; got != first {
			t.Fatalf("concurrent retries diverged: %s vs %s", first, got)
		}
	}
	if c := p.calls.Load(); c != 1 {
		t.Fatalf("expected exactly one execution, got %d", c)
	}
}

func TestReadyWithMemoryStores(t *testing.T) {
	if err := newTestKernel(t).Ready(context.Background()); err != nil {
		t.Fatalf("memory-backed kernel should be ready: %v", err)
	}
}
