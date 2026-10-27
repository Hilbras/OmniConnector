package kernel

import (
	"sort"
	"sync"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// Operation is the kernel's record of one execution request. It deliberately
// holds no operation input: inputs may carry sensitive values and only their
// validation outcome is part of the record.
type Operation struct {
	ID              string                   `json:"id"`
	RequestID       string                   `json:"requestId,omitempty"`
	Integration     string                   `json:"integration"`
	OperationID     string                   `json:"operationId"`
	Capability      string                   `json:"capability"`
	ConnectionID    string                   `json:"connectionId,omitempty"`
	CallerUserID    string                   `json:"callerUserId,omitempty"`
	CallerAppID     string                   `json:"callerAppId,omitempty"`
	ExecutionTarget string                   `json:"executionTarget"`
	Authorization   string                   `json:"authorization"`
	IdempotencyKey  string                   `json:"idempotencyKey,omitempty"`
	Idempotent      bool                     `json:"idempotent"`
	TimeoutSeconds  int                      `json:"timeoutSeconds"`
	State           contracts.OperationState `json:"state"`
	Result          map[string]any           `json:"result,omitempty"`
	ResultMetadata  map[string]any           `json:"resultMetadata,omitempty"`
	Error           *contracts.Error         `json:"error,omitempty"`
	CreatedAt       time.Time                `json:"createdAt"`
	StartedAt       *time.Time               `json:"startedAt,omitempty"`
	FinishedAt      *time.Time               `json:"finishedAt,omitempty"`
}

// Terminal reports whether the operation has reached a final state
func (o *Operation) Terminal() bool {
	switch o.State {
	case contracts.StateSucceeded, contracts.StateFailed, contracts.StateCancelled,
		contracts.StateTimedOut, contracts.StateUnknownOutcome:
		return true
	}
	return false
}

// OperationFilter narrows a List call
type OperationFilter struct {
	CallerUserID string
	CallerAppID  string
	Limit        int
}

// OperationStore persists operation records. The in-memory implementation
// backs Phase 1; a PostgreSQL implementation replaces it without changing
// the dispatcher.
type OperationStore interface {
	Create(op *Operation) error
	Update(op *Operation) error
	Get(id string) (*Operation, bool)
	// FindByIdempotencyKey returns the operation a caller previously created
	// with the same key, so a retried request returns the original outcome.
	FindByIdempotencyKey(callerUserID, callerAppID, key string) (*Operation, bool)
	List(filter OperationFilter) []*Operation
}

// MemoryOperationStore is an in-process OperationStore. Records are copied in
// and out so callers can never mutate stored state by accident.
type MemoryOperationStore struct {
	mu    sync.RWMutex
	ops   map[string]*Operation
	idemp map[string]string
}

// NewMemoryOperationStore creates an empty in-memory store
func NewMemoryOperationStore() *MemoryOperationStore {
	return &MemoryOperationStore{ops: make(map[string]*Operation), idemp: make(map[string]string)}
}

func idempotencyIndexKey(userID, appID, key string) string {
	return userID + "\x00" + appID + "\x00" + key
}

// Create stores a new operation
func (s *MemoryOperationStore) Create(op *Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *op
	s.ops[op.ID] = &cp
	if op.IdempotencyKey != "" {
		s.idemp[idempotencyIndexKey(op.CallerUserID, op.CallerAppID, op.IdempotencyKey)] = op.ID
	}
	return nil
}

// Update replaces a stored operation
func (s *MemoryOperationStore) Update(op *Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *op
	s.ops[op.ID] = &cp
	return nil
}

// Get returns a copy of the operation with the given ID
func (s *MemoryOperationStore) Get(id string) (*Operation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[id]
	if !ok {
		return nil, false
	}
	cp := *op
	return &cp, true
}

// FindByIdempotencyKey looks up a prior operation by caller and key
func (s *MemoryOperationStore) FindByIdempotencyKey(userID, appID, key string) (*Operation, bool) {
	s.mu.RLock()
	id, ok := s.idemp[idempotencyIndexKey(userID, appID, key)]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return s.Get(id)
}

// List returns matching operations, newest first
func (s *MemoryOperationStore) List(f OperationFilter) []*Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Operation, 0)
	for _, op := range s.ops {
		if f.CallerUserID != "" && op.CallerUserID != f.CallerUserID {
			continue
		}
		if f.CallerAppID != "" && op.CallerAppID != f.CallerAppID {
			continue
		}
		cp := *op
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out
}
