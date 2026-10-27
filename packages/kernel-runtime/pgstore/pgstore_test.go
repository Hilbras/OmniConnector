package pgstore

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/ids"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/storetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool returns a pool confined to a fresh, migrated schema that is
// dropped when the test ends. Tests skip unless KERNEL_TEST_DATABASE_DSN is set.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("KERNEL_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("KERNEL_TEST_DATABASE_DSN not set; skipping PostgreSQL tests")
	}
	ctx := context.Background()

	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(ids.New(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func TestPostgresOperationStore(t *testing.T) {
	storetest.OperationStore(t, func(t *testing.T) kernel.OperationStore { return NewOperationStore(testPool(t)) })
}

func TestPostgresAuditStore(t *testing.T) {
	storetest.AuditStore(t, func(t *testing.T) audit.Store { return NewAuditStore(testPool(t)) })
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testPool(t)
	ran, err := Migrate(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(ran) != 0 {
		t.Fatalf("second run applied %v", ran)
	}
}

func TestMigrationsAreWellFormed(t *testing.T) {
	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range ms {
		if m.version != i+1 {
			t.Fatalf("migration versions must be contiguous from 1: %s is version %d at position %d", m.name, m.version, i+1)
		}
	}
}

func TestAuditEventsAreImmutable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	rec := audit.NewWithStore(NewAuditStore(pool))
	if err := rec.Record(ctx, audit.Event{Actor: "u1", ActorType: "user", Action: "a", TargetType: "t", TargetID: "x", Outcome: "ok"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE audit_events SET outcome = 'tampered'"); err == nil {
		t.Fatal("updating an audit event must fail")
	}
}

func TestOpenRedactsDSN(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:s3cret@[bad")
	if err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("Open must not echo the DSN password: %v", err)
	}
}

// TestKernelOnPostgres runs a real operation through the kernel with both
// stores on PostgreSQL and reads the persisted outcome back.
func TestKernelOnPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	k := newPGKernel(t, pool)

	op, err := submitEcho(k)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := NewOperationStore(pool).Get(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != "succeeded" || stored.Result["echo"] == nil || stored.FinishedAt == nil {
		t.Fatalf("persisted operation incomplete: %+v", stored)
	}
	events, err := NewAuditStore(pool).List(ctx, audit.Filter{Actor: "u1"})
	if err != nil || len(events) != 1 || events[0].Outcome != "succeeded" {
		t.Fatalf("expected one persisted audit event, got %v, %v", events, err)
	}
	if err := k.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	pool.Close()
	if err := k.Ready(ctx); err == nil {
		t.Fatal("Ready must fail once the database is unreachable")
	}
}
