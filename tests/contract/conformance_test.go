// Package contract holds the connector conformance suite. Every integration
// in the repository must pass it before the kernel will be asked to load it.
package contract

import (
	"context"
	"io"
	"reflect"
	"sort"
	"testing"

	"github.com/hilbras/omniconnector/integrations/github"
	"github.com/hilbras/omniconnector/integrations/sftp"
	"github.com/hilbras/omniconnector/integrations/ssh"
	testintegration "github.com/hilbras/omniconnector/integrations/test-integration"
	"github.com/hilbras/omniconnector/integrations/vercel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// integrations lists every package under test. Add new integrations here.
func integrations() []contracts.Package {
	return []contracts.Package{
		testintegration.NewIntegration(),
		github.NewIntegration(),
		vercel.NewIntegration(),
		ssh.NewIntegration(),
		sftp.NewIntegration(),
	}
}

func newKernel(t *testing.T) *kernel.Kernel {
	t.Helper()
	cfg := &config.Config{Execution: config.ExecutionConfig{MaxConcurrent: 2, Timeout: 5, MaxOutputBytes: 1 << 20}}
	k, err := kernel.New(cfg, logger.NewWithWriter(io.Discard, "error"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Shutdown(context.Background()) })
	return k
}

func TestConformance(t *testing.T) {
	for _, pkg := range integrations() {
		m := pkg.Manifest()
		if m == nil {
			t.Fatalf("%T returned a nil manifest", pkg)
		}
		t.Run(m.Metadata.Name, func(t *testing.T) {
			t.Run("manifest is valid", func(t *testing.T) {
				if err := kernel.ValidateManifest(m); err != nil {
					t.Fatal(err)
				}
			})

			t.Run("registers with the kernel", func(t *testing.T) {
				if err := newKernel(t).RegisterPackage(pkg); err != nil {
					t.Fatal(err)
				}
			})

			t.Run("runtime capabilities match manifest", func(t *testing.T) {
				if !reflect.DeepEqual(pkg.Capabilities(), m.Spec.Capabilities) {
					t.Fatalf("Capabilities() = %v, manifest declares %v", pkg.Capabilities(), m.Spec.Capabilities)
				}
			})

			t.Run("auth methods match manifest", func(t *testing.T) {
				got := authStrings(pkg.SupportedAuthMethods())
				want := authStrings(m.Spec.AuthMethods)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("SupportedAuthMethods() = %v, manifest declares %v", got, want)
				}
			})

			t.Run("every declared capability has an operation", func(t *testing.T) {
				used := map[string]bool{}
				for _, op := range m.Spec.Operations {
					used[op.Capability] = true
				}
				for _, c := range m.Spec.Capabilities {
					if !used[c.Name] {
						t.Errorf("capability %q has no operation", c.Name)
					}
				}
			})

			t.Run("unknown operation fails with a normalized error", func(t *testing.T) {
				res, err := pkg.Execute(context.Background(), &contracts.OperationRequest{OperationID: "conformance.nonexistent", Input: map[string]any{}})
				if err == nil && (res == nil || res.Error == nil) {
					t.Fatal("unknown operation must not succeed")
				}
				if err != nil {
					if _, ok := contracts.AsError(err); !ok {
						t.Fatalf("expected *contracts.Error, got %T: %v", err, err)
					}
				}
			})

			t.Run("shutdown is clean", func(t *testing.T) {
				if err := pkg.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func authStrings(ms []contracts.AuthMethod) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = string(m)
	}
	sort.Strings(out)
	return out
}
