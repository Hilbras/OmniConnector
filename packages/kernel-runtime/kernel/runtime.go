package kernel

import (
	"context"
	"sync"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

const progressBuffer = 64

// runtimeContext is the kernel surface handed to one package. Each package
// gets its own so audit events and logs are attributed to it and cannot be
// forged as coming from another actor.
type runtimeContext struct {
	k         *Kernel
	name      string
	log       *logger.Logger
	progress  chan contracts.OperationProgress
	closeOnce sync.Once
}

func newRuntimeContext(k *Kernel, name string) *runtimeContext {
	rc := &runtimeContext{
		k:        k,
		name:     name,
		log:      k.log.With("integration", name),
		progress: make(chan contracts.OperationProgress, progressBuffer),
	}
	go rc.drainProgress()
	return rc
}

// Logger returns a logger scoped to the package
func (rc *runtimeContext) Logger() contracts.Logger { return rc.log }

// GetCredential resolves credential material. Credential references arrive
// with connection management in Phase 2; until then nothing resolves.
func (rc *runtimeContext) GetCredential(ctx context.Context, ref contracts.CredentialRef) (*contracts.CredentialMaterial, error) {
	return nil, contracts.NewError(contracts.ErrorCategoryNotSupported, "credentials_unavailable", "credential resolution is not available yet")
}

// EmitAuditEvent records an event attributed to the package. The package's
// claimed actor is kept as metadata, never trusted as the event actor.
func (rc *runtimeContext) EmitAuditEvent(ctx context.Context, ev contracts.AuditEvent) error {
	meta := make(map[string]any, len(ev.Metadata)+1)
	for k, v := range ev.Metadata {
		meta[k] = v
	}
	if ev.Actor != "" {
		meta["reportedActor"] = ev.Actor
	}
	if rid := RequestID(ctx); rid != "" {
		meta["requestId"] = rid
	}
	rc.k.auditRec.Record(ctx, audit.Event{
		Actor:      rc.name,
		ActorType:  "integration",
		Action:     ev.Action,
		TargetType: "resource",
		TargetID:   ev.Target,
		Outcome:    ev.Outcome,
		Metadata:   meta,
		Timestamp:  ev.Timestamp,
	})
	return nil
}

// ProgressChan returns the package's progress channel. Packages must not
// send after their Shutdown returns.
func (rc *runtimeContext) ProgressChan() chan contracts.OperationProgress { return rc.progress }

// drainProgress keeps the channel flowing. Progress fan-out to clients
// (SSE, ADR-010) attaches here.
func (rc *runtimeContext) drainProgress() {
	for p := range rc.progress {
		rc.log.Debug("operation progress", "operation_id", p.OperationID, "progress", p.Progress, "message", p.Message)
	}
}

func (rc *runtimeContext) close() {
	rc.closeOnce.Do(func() { close(rc.progress) })
}
