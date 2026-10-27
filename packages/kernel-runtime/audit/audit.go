package audit

import (
	"context"
	"sync"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/ids"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// Event represents an audit event
type Event struct {
	ID         string         `json:"id"`
	Actor      string         `json:"actor"`
	ActorType  string         `json:"actorType"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Outcome    string         `json:"outcome"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Recorder records audit events
type Recorder struct {
	mu     sync.RWMutex
	events []Event
}

// New creates a new audit recorder
func New() *Recorder {
	return &Recorder{events: make([]Event, 0)}
}

// Record records an audit event
func (r *Recorder) Record(ctx context.Context, event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.ID == "" {
		event.ID = ids.WithPrefix("aud")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	r.events = append(r.events, event)
}

// RecordFromContract records an audit event from a shared-contracts AuditEvent
func (r *Recorder) RecordFromContract(ctx context.Context, ae contracts.AuditEvent) {
	r.Record(ctx, Event{
		Actor:      ae.Actor,
		Action:     ae.Action,
		TargetType: "resource",
		TargetID:   ae.Target,
		Outcome:    ae.Outcome,
		Metadata:   ae.Metadata,
		Timestamp:  ae.Timestamp,
	})
}

// List returns all recorded events
func (r *Recorder) List() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Event, len(r.events))
	copy(result, r.events)
	return result
}

// ListByActor returns events for a specific actor
func (r *Recorder) ListByActor(actor string) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Event
	for _, e := range r.events {
		if e.Actor == actor {
			result = append(result, e)
		}
	}
	return result
}

// Count returns the number of recorded events
func (r *Recorder) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
