package kernel

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/auth"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/connection"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/credential"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/permission"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

const (
	// Version is the kernel build version reported by health endpoints
	Version = "0.1.0"
	// RuntimeVersion is matched against a manifest's supportedRuntimeVersions
	RuntimeVersion = "v1.0"
	// PackageAPIVersion is the integration contract version this kernel speaks
	PackageAPIVersion = "v1"
	// ManifestAPIVersion is the manifest schema version this kernel accepts
	ManifestAPIVersion = "omni/v1"

	lifecycleTimeout = 10 * time.Second
)

// registeredPackage is a package together with the kernel-side state built
// from its manifest at registration time
type registeredPackage struct {
	pkg          contracts.Package
	manifest     *contracts.Manifest
	operations   map[string]contracts.OperationDef
	capabilities map[string]contracts.Capability
	runtime      *runtimeContext
}

// Kernel is the main kernel runtime
type Kernel struct {
	cfg        *config.Config
	log        *logger.Logger
	connMgr    *connection.Manager
	credStore  *credential.Store
	tokenMgr   *auth.TokenManager
	permEngine *permission.Engine
	auditRec   *audit.Recorder
	ops        OperationStore
	slots      chan struct{}
	startedAt  time.Time

	mu      sync.RWMutex
	pkgs    map[string]*registeredPackage
	running map[string]*runningOp
}

// Option configures a Kernel
type Option func(*Kernel)

// WithOperationStore replaces the default in-memory operation store
func WithOperationStore(s OperationStore) Option {
	return func(k *Kernel) { k.ops = s }
}

// WithAuditStore replaces the default in-memory audit store
func WithAuditStore(s audit.Store) Option {
	return func(k *Kernel) { k.auditRec = audit.NewWithStore(s) }
}

// New creates a new kernel
func New(cfg *config.Config, log *logger.Logger, opts ...Option) (*Kernel, error) {
	encKey := cfg.CredentialKeyBytes()
	if encKey == nil {
		encKey = make([]byte, 32)
		if _, err := rand.Read(encKey); err != nil {
			return nil, fmt.Errorf("failed to generate credential key: %w", err)
		}
		log.Warn("no credential key configured; using an ephemeral key, stored credentials will not survive restart")
	}
	credStore, err := credential.NewStore(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}

	maxConcurrent := cfg.Execution.MaxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	k := &Kernel{
		cfg:        cfg,
		log:        log,
		pkgs:       make(map[string]*registeredPackage),
		running:    make(map[string]*runningOp),
		connMgr:    connection.New(),
		credStore:  credStore,
		tokenMgr:   auth.New(),
		permEngine: permission.New(),
		auditRec:   audit.New(),
		ops:        NewMemoryOperationStore(),
		slots:      make(chan struct{}, maxConcurrent),
		startedAt:  time.Now(),
	}
	for _, opt := range opts {
		opt(k)
	}
	return k, nil
}

// Start starts the kernel
func (k *Kernel) Start() error {
	k.log.Info("kernel started", "version", Version, "runtime_version", RuntimeVersion, "packages", len(k.pkgs))
	return nil
}

// Shutdown cancels running operations and shuts down every registered package
func (k *Kernel) Shutdown(ctx context.Context) error {
	k.log.Info("kernel shutting down")

	k.mu.Lock()
	for _, entry := range k.running {
		entry.cancel()
	}
	pkgs := k.pkgs
	k.pkgs = make(map[string]*registeredPackage)
	k.mu.Unlock()

	var firstErr error
	for name, rp := range pkgs {
		sctx, cancel := context.WithTimeout(ctx, lifecycleTimeout)
		if err := rp.pkg.Shutdown(sctx); err != nil {
			k.log.Error("package shutdown failed", "integration", name, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("shutdown %s: %w", name, err)
			}
		}
		cancel()
		rp.runtime.close()
	}
	return firstErr
}

// RegisterPackage validates a package's manifest, initializes it, and makes
// its operations available for dispatch
func (k *Kernel) RegisterPackage(pkg contracts.Package) error {
	manifest := pkg.Manifest()
	if manifest == nil {
		return fmt.Errorf("package manifest is nil")
	}
	if err := ValidateManifest(manifest); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}
	name := manifest.Metadata.Name

	k.mu.RLock()
	_, exists := k.pkgs[name]
	k.mu.RUnlock()
	if exists {
		return fmt.Errorf("package already registered: %s", name)
	}

	rp := &registeredPackage{
		pkg:          pkg,
		manifest:     manifest,
		operations:   make(map[string]contracts.OperationDef, len(manifest.Spec.Operations)),
		capabilities: make(map[string]contracts.Capability, len(manifest.Spec.Capabilities)),
		runtime:      newRuntimeContext(k, name),
	}
	for _, c := range manifest.Spec.Capabilities {
		rp.capabilities[c.Name] = c
	}
	for _, op := range manifest.Spec.Operations {
		rp.operations[op.ID] = op
	}

	ctx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()
	if err := pkg.Initialize(ctx, rp.runtime); err != nil {
		rp.runtime.close()
		return fmt.Errorf("initialize %s: %w", name, err)
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	if _, exists := k.pkgs[name]; exists {
		rp.runtime.close()
		return fmt.Errorf("package already registered: %s", name)
	}
	k.pkgs[name] = rp
	k.log.Info("package registered", "integration", name, "version", manifest.Metadata.Version, "operations", len(rp.operations))
	return nil
}

// GetPackage returns a registered package by name
func (k *Kernel) GetPackage(name string) (contracts.Package, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	rp, ok := k.pkgs[name]
	if !ok {
		return nil, false
	}
	return rp.pkg, true
}

// ListPackages returns all registered packages sorted by name
func (k *Kernel) ListPackages() []contracts.Package {
	k.mu.RLock()
	defer k.mu.RUnlock()
	names := make([]string, 0, len(k.pkgs))
	for name := range k.pkgs {
		names = append(names, name)
	}
	sort.Strings(names)
	pkgs := make([]contracts.Package, 0, len(names))
	for _, name := range names {
		pkgs = append(pkgs, k.pkgs[name].pkg)
	}
	return pkgs
}

// GetCapabilities returns the declared capabilities of every package. The
// manifest is the source of truth, not the package's runtime self-report.
func (k *Kernel) GetCapabilities() map[string][]contracts.Capability {
	k.mu.RLock()
	defer k.mu.RUnlock()
	caps := make(map[string][]contracts.Capability, len(k.pkgs))
	for name, rp := range k.pkgs {
		caps[name] = rp.manifest.Spec.Capabilities
	}
	return caps
}

// ConnectionManager returns the connection manager
func (k *Kernel) ConnectionManager() *connection.Manager {
	return k.connMgr
}

// CredentialStore returns the credential store
func (k *Kernel) CredentialStore() *credential.Store {
	return k.credStore
}

// TokenManager returns the token manager
func (k *Kernel) TokenManager() *auth.TokenManager {
	return k.tokenMgr
}

// PermissionEngine returns the permission engine
func (k *Kernel) PermissionEngine() *permission.Engine {
	return k.permEngine
}

// AuditRecorder returns the audit recorder
func (k *Kernel) AuditRecorder() *audit.Recorder {
	return k.auditRec
}

// Ready reports whether the kernel can serve traffic: every store that can
// be pinged must answer
func (k *Kernel) Ready(ctx context.Context) error {
	for _, s := range []any{k.ops, k.auditStore()} {
		if p, ok := s.(interface{ Ping(context.Context) error }); ok {
			if err := p.Ping(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *Kernel) auditStore() any { return k.auditRec.Store() }

// Health returns the kernel health status
func (k *Kernel) Health() map[string]interface{} {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return map[string]interface{}{
		"status":            "healthy",
		"version":           Version,
		"runtimeVersion":    RuntimeVersion,
		"uptimeSeconds":     int64(time.Since(k.startedAt).Seconds()),
		"packages":          len(k.pkgs),
		"runningOperations": len(k.running),
	}
}
