package audit

import (
	"context"
	"sync"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/ids"
)

// Event represents an audit event. It must never carry secrets: no
// passwords, tokens, keys, or raw operation input.
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

// Filter narrows a List call. Zero values mean "no constraint".
type Filter struct {
	Actor string
	Limit int
}

// Store persists audit events. Events are append-only.
type Store interface {
	Append(ctx context.Context, e Event) error
	// List returns matching events, newest first
	List(ctx context.Context, f Filter) ([]Event, error)
}

// Recorder stamps events with an ID and timestamp and appends them to a Store
type Recorder struct {
	store Store
}

// New creates a recorder backed by an in-memory store
func New() *Recorder {
	return NewWithStore(NewMemoryStore())
}

// NewWithStore creates a recorder backed by the given store
func NewWithStore(s Store) *Recorder {
	return &Recorder{store: s}
}

// Record appends an audit event
func (r *Recorder) Record(ctx context.Context, event Event) error {
	if event.ID == "" {
		event.ID = ids.WithPrefix("aud")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	return r.store.Append(ctx, event)
}

// Store returns the underlying store
func (r *Recorder) Store() Store { return r.store }

// List returns matching events, newest first
func (r *Recorder) List(ctx context.Context, f Filter) ([]Event, error) {
	return r.store.List(ctx, f)
}

// MemoryStore is an in-process Store for tests and local development
type MemoryStore struct {
	mu     sync.RWMutex
	events []Event
}

// NewMemoryStore creates an empty in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Append stores an event
func (m *MemoryStore) Append(ctx context.Context, e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// List returns matching events, newest first
func (m *MemoryStore) List(ctx context.Context, f Filter) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Event, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		if f.Actor != "" && e.Actor != f.Actor {
			continue
		}
		out = append(out, e)
		if f.Limit > 0 && len(out) == f.Limit {
			break
		}
	}
	return out, nil
}
