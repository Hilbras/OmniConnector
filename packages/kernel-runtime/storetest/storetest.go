// Package storetest is a behavioral suite every kernel.OperationStore and
// audit.Store implementation must pass, so the in-memory and PostgreSQL
// stores cannot drift apart.
package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/ids"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// newOp builds a queued operation; PostgreSQL keeps microseconds, so
// timestamps are truncated to make round-trips comparable.
func newOp(user, key string, created time.Time) *kernel.Operation {
	return &kernel.Operation{
		ID:              ids.WithPrefix("op"),
		RequestID:       "req-1",
		Integration:     "test-integration",
		OperationID:     "echo",
		Capability:      "test.read",
		CallerUserID:    user,
		ExecutionTarget: "cloud",
		Authorization:   "all required permissions granted",
		IdempotencyKey:  key,
		Idempotent:      true,
		TimeoutSeconds:  5,
		State:           contracts.StateQueued,
		CreatedAt:       created.UTC().Truncate(time.Microsecond),
	}
}

// OperationStore runs the operation store suite. newStore must return an
// empty store each call.
func OperationStore(t *testing.T, newStore func(t *testing.T) kernel.OperationStore) {
	ctx := context.Background()

	t.Run("create get roundtrip", func(t *testing.T) {
		s := newStore(t)
		op := newOp("u1", "", time.Now())
		if err := s.Create(ctx, op); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, op.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != op.ID || got.State != contracts.StateQueued || got.CallerUserID != "u1" ||
			got.Capability != "test.read" || !got.Idempotent || got.TimeoutSeconds != 5 || !got.CreatedAt.Equal(op.CreatedAt) {
			t.Fatalf("roundtrip mismatch:\n got  %+v\n want %+v", got, op)
		}
		if got.StartedAt != nil || got.FinishedAt != nil || got.Result != nil || got.Error != nil {
			t.Fatalf("unset fields must stay unset: %+v", got)
		}
	})

	t.Run("get missing", func(t *testing.T) {
		if _, err := newStore(t).Get(ctx, "op_missing"); !errors.Is(err, kernel.ErrOperationNotFound) {
			t.Fatalf("expected ErrOperationNotFound, got %v", err)
		}
	})

	t.Run("update persists outcome", func(t *testing.T) {
		s := newStore(t)
		op := newOp("u1", "", time.Now())
		if err := s.Create(ctx, op); err != nil {
			t.Fatal(err)
		}
		started := time.Now().UTC().Truncate(time.Microsecond)
		finished := started.Add(time.Second)
		op.State = contracts.StateFailed
		op.StartedAt, op.FinishedAt = &started, &finished
		op.Error = contracts.NewError(contracts.ErrorCategoryRateLimited, "slow_down", "rate limited")
		op.Result = map[string]any{"n": float64(3), "nested": map[string]any{"ok": true}}
		op.ResultMetadata = map[string]any{"page": "2"}
		if err := s.Update(ctx, op); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, op.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != contracts.StateFailed || !got.StartedAt.Equal(started) || !got.FinishedAt.Equal(finished) {
			t.Fatalf("state/timestamps not persisted: %+v", got)
		}
		if got.Error == nil || got.Error.Code != "slow_down" || got.Error.Category != contracts.ErrorCategoryRateLimited || !got.Error.Retryable {
			t.Fatalf("error not persisted: %+v", got.Error)
		}
		if got.Result["n"] != float64(3) || got.Result["nested"].(map[string]any)["ok"] != true || got.ResultMetadata["page"] != "2" {
			t.Fatalf("result not persisted: %+v %+v", got.Result, got.ResultMetadata)
		}
	})

	t.Run("update missing", func(t *testing.T) {
		if err := newStore(t).Update(ctx, newOp("u1", "", time.Now())); !errors.Is(err, kernel.ErrOperationNotFound) {
			t.Fatalf("expected ErrOperationNotFound, got %v", err)
		}
	})

	t.Run("idempotency keys", func(t *testing.T) {
		s := newStore(t)
		first := newOp("u1", "k1", time.Now())
		if err := s.Create(ctx, first); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(ctx, newOp("u1", "k1", time.Now())); !errors.Is(err, kernel.ErrDuplicateIdempotencyKey) {
			t.Fatalf("expected ErrDuplicateIdempotencyKey, got %v", err)
		}
		if err := s.Create(ctx, newOp("u2", "k1", time.Now())); err != nil {
			t.Fatalf("keys must be scoped per caller: %v", err)
		}
		for i := 0; i < 2; i++ {
			if err := s.Create(ctx, newOp("u1", "", time.Now())); err != nil {
				t.Fatalf("empty keys must never collide: %v", err)
			}
		}
		got, err := s.FindByIdempotencyKey(ctx, "u1", "", "k1")
		if err != nil || got.ID != first.ID {
			t.Fatalf("FindByIdempotencyKey = %v, %v; want %s", got, err, first.ID)
		}
		if _, err := s.FindByIdempotencyKey(ctx, "u1", "", "missing"); !errors.Is(err, kernel.ErrOperationNotFound) {
			t.Fatalf("expected ErrOperationNotFound, got %v", err)
		}
		if _, err := s.FindByIdempotencyKey(ctx, "u1", "", ""); !errors.Is(err, kernel.ErrOperationNotFound) {
			t.Fatalf("the empty key must never match, got %v", err)
		}
	})

	t.Run("list filters and orders newest first", func(t *testing.T) {
		s := newStore(t)
		base := time.Now().Add(-time.Hour)
		var u1 []string
		for i := 0; i < 3; i++ {
			op := newOp("u1", "", base.Add(time.Duration(i)*time.Minute))
			u1 = append(u1, op.ID)
			if err := s.Create(ctx, op); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Create(ctx, newOp("u2", "", base)); err != nil {
			t.Fatal(err)
		}

		all, err := s.List(ctx, kernel.OperationFilter{})
		if err != nil || len(all) != 4 {
			t.Fatalf("List all = %d, %v", len(all), err)
		}
		mine, err := s.List(ctx, kernel.OperationFilter{CallerUserID: "u1"})
		if err != nil || len(mine) != 3 {
			t.Fatalf("List u1 = %d, %v", len(mine), err)
		}
		if mine[0].ID != u1[2] || mine[2].ID != u1[0] {
			t.Fatalf("expected newest first, got %s, %s, %s", mine[0].ID, mine[1].ID, mine[2].ID)
		}
		limited, err := s.List(ctx, kernel.OperationFilter{CallerUserID: "u1", Limit: 2})
		if err != nil || len(limited) != 2 || limited[0].ID != u1[2] {
			t.Fatalf("limit not applied: %d, %v", len(limited), err)
		}
		none, err := s.List(ctx, kernel.OperationFilter{CallerUserID: "nobody"})
		if err != nil || none == nil || len(none) != 0 {
			t.Fatalf("empty result must be a non-nil empty slice, got %v, %v", none, err)
		}
	})

	t.Run("returned records are copies", func(t *testing.T) {
		s := newStore(t)
		op := newOp("u1", "", time.Now())
		if err := s.Create(ctx, op); err != nil {
			t.Fatal(err)
		}
		op.State = contracts.StateSucceeded
		got, _ := s.Get(ctx, op.ID)
		got.State = contracts.StateFailed
		again, _ := s.Get(ctx, op.ID)
		if again.State != contracts.StateQueued {
			t.Fatalf("store state changed without Update: %s", again.State)
		}
	})
}

// AuditStore runs the audit store suite. newStore must return an empty store each call.
func AuditStore(t *testing.T, newStore func(t *testing.T) audit.Store) {
	ctx := context.Background()

	t.Run("append and list newest first", func(t *testing.T) {
		rec := audit.NewWithStore(newStore(t))
		for _, actor := range []string{"u1", "u2", "u1"} {
			if err := rec.Record(ctx, audit.Event{
				Actor: actor, ActorType: "user", Action: "operation.execute",
				TargetType: "operation", TargetID: "test-integration/echo", Outcome: "succeeded",
				Metadata: map[string]any{"operationId": "op_1", "seq": float64(1)},
			}); err != nil {
				t.Fatal(err)
			}
		}
		all, err := rec.List(ctx, audit.Filter{})
		if err != nil || len(all) != 3 {
			t.Fatalf("List = %d, %v", len(all), err)
		}
		if all[0].ID == "" || all[0].Timestamp.IsZero() {
			t.Fatalf("recorder must assign ID and timestamp: %+v", all[0])
		}
		if all[0].Metadata["operationId"] != "op_1" || all[0].TargetID != "test-integration/echo" {
			t.Fatalf("event fields not persisted: %+v", all[0])
		}
		if !all[0].Timestamp.After(all[2].Timestamp) && !all[0].Timestamp.Equal(all[2].Timestamp) {
			t.Fatalf("expected newest first")
		}

		u1, err := rec.List(ctx, audit.Filter{Actor: "u1"})
		if err != nil || len(u1) != 2 {
			t.Fatalf("actor filter = %d, %v", len(u1), err)
		}
		one, err := rec.List(ctx, audit.Filter{Limit: 1})
		if err != nil || len(one) != 1 || one[0].ID != all[0].ID {
			t.Fatalf("limit = %v, %v", one, err)
		}
	})

	t.Run("empty list is non-nil", func(t *testing.T) {
		got, err := newStore(t).List(ctx, audit.Filter{Actor: "nobody"})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("expected empty non-nil slice, got %v, %v", got, err)
		}
	})
}
