// Package pgstore implements the kernel's operation and audit stores on PostgreSQL.
package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505"

// Open connects to PostgreSQL and verifies the connection
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		// The parse error can echo the DSN, password included
		return nil, errors.New("invalid database DSN")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// OperationStore is a kernel.OperationStore backed by PostgreSQL
type OperationStore struct {
	pool *pgxpool.Pool
}

// NewOperationStore creates an operation store on pool
func NewOperationStore(pool *pgxpool.Pool) *OperationStore {
	return &OperationStore{pool: pool}
}

var _ kernel.OperationStore = (*OperationStore)(nil)

// Ping checks database connectivity
func (s *OperationStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

const operationColumns = `id, request_id, integration, operation_id, capability, connection_id,
	caller_user_id, caller_app_id, execution_target, authorization_decision, idempotency_key,
	idempotent, timeout_seconds, state, result, result_metadata, error, created_at, started_at, finished_at`

// Create inserts a new operation
func (s *OperationStore) Create(ctx context.Context, op *kernel.Operation) error {
	result, resultMeta, opErr, err := encodeOutcome(op)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO operations (`+operationColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		op.ID, op.RequestID, op.Integration, op.OperationID, op.Capability, op.ConnectionID,
		op.CallerUserID, op.CallerAppID, op.ExecutionTarget, op.Authorization, op.IdempotencyKey,
		op.Idempotent, op.TimeoutSeconds, string(op.State), result, resultMeta, opErr,
		op.CreatedAt, op.StartedAt, op.FinishedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == "operations_idempotency_key" {
		return kernel.ErrDuplicateIdempotencyKey
	}
	if err != nil {
		return fmt.Errorf("insert operation: %w", err)
	}
	return nil
}

// Update writes the mutable fields of an operation: state, outcome, and timestamps
func (s *OperationStore) Update(ctx context.Context, op *kernel.Operation) error {
	result, resultMeta, opErr, err := encodeOutcome(op)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE operations
		SET state = $2, result = $3, result_metadata = $4, error = $5,
		    started_at = $6, finished_at = $7, updated_at = now()
		WHERE id = $1`,
		op.ID, string(op.State), result, resultMeta, opErr, op.StartedAt, op.FinishedAt)
	if err != nil {
		return fmt.Errorf("update operation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return kernel.ErrOperationNotFound
	}
	return nil
}

// Get returns the operation with the given ID
func (s *OperationStore) Get(ctx context.Context, id string) (*kernel.Operation, error) {
	return scanOperation(s.pool.QueryRow(ctx, `SELECT `+operationColumns+` FROM operations WHERE id = $1`, id))
}

// FindByIdempotencyKey returns the caller's operation with the given key
func (s *OperationStore) FindByIdempotencyKey(ctx context.Context, userID, appID, key string) (*kernel.Operation, error) {
	return scanOperation(s.pool.QueryRow(ctx, `SELECT `+operationColumns+` FROM operations
		WHERE caller_user_id = $1 AND caller_app_id = $2 AND idempotency_key = $3 AND idempotency_key <> ''`,
		userID, appID, key))
}

// List returns matching operations, newest first
func (s *OperationStore) List(ctx context.Context, f kernel.OperationFilter) ([]*kernel.Operation, error) {
	query := `SELECT ` + operationColumns + ` FROM operations WHERE TRUE`
	var args []any
	if f.CallerUserID != "" {
		args = append(args, f.CallerUserID)
		query += fmt.Sprintf(` AND caller_user_id = $%d`, len(args))
	}
	if f.CallerAppID != "" {
		args = append(args, f.CallerAppID)
		query += fmt.Sprintf(` AND caller_app_id = $%d`, len(args))
	}
	query += ` ORDER BY created_at DESC, id DESC`
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(` LIMIT $%d`, len(args))
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	defer rows.Close()
	out := make([]*kernel.Operation, 0)
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

func encodeOutcome(op *kernel.Operation) (result, resultMeta, opErr []byte, err error) {
	if result, err = marshalNullable(op.Result); err != nil {
		return nil, nil, nil, fmt.Errorf("encode result: %w", err)
	}
	if resultMeta, err = marshalNullable(op.ResultMetadata); err != nil {
		return nil, nil, nil, fmt.Errorf("encode result metadata: %w", err)
	}
	if op.Error != nil {
		if opErr, err = json.Marshal(op.Error); err != nil {
			return nil, nil, nil, fmt.Errorf("encode error: %w", err)
		}
	}
	return result, resultMeta, opErr, nil
}

func marshalNullable(m map[string]any) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func scanOperation(row pgx.Row) (*kernel.Operation, error) {
	var (
		op                        kernel.Operation
		state                     string
		result, resultMeta, opErr []byte
		createdAt                 time.Time
		startedAt, finishedAt     *time.Time
	)
	err := row.Scan(&op.ID, &op.RequestID, &op.Integration, &op.OperationID, &op.Capability, &op.ConnectionID,
		&op.CallerUserID, &op.CallerAppID, &op.ExecutionTarget, &op.Authorization, &op.IdempotencyKey,
		&op.Idempotent, &op.TimeoutSeconds, &state, &result, &resultMeta, &opErr, &createdAt, &startedAt, &finishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, kernel.ErrOperationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan operation: %w", err)
	}
	op.State = contracts.OperationState(state)
	op.CreatedAt = createdAt.UTC()
	op.StartedAt = utcPtr(startedAt)
	op.FinishedAt = utcPtr(finishedAt)
	if result != nil {
		if err := json.Unmarshal(result, &op.Result); err != nil {
			return nil, fmt.Errorf("decode result: %w", err)
		}
	}
	if resultMeta != nil {
		if err := json.Unmarshal(resultMeta, &op.ResultMetadata); err != nil {
			return nil, fmt.Errorf("decode result metadata: %w", err)
		}
	}
	if opErr != nil {
		op.Error = &contracts.Error{}
		if err := json.Unmarshal(opErr, op.Error); err != nil {
			return nil, fmt.Errorf("decode error: %w", err)
		}
	}
	return &op, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// AuditStore is an audit.Store backed by PostgreSQL
type AuditStore struct {
	pool *pgxpool.Pool
}

// NewAuditStore creates an audit store on pool
func NewAuditStore(pool *pgxpool.Pool) *AuditStore {
	return &AuditStore{pool: pool}
}

var _ audit.Store = (*AuditStore)(nil)

// Ping checks database connectivity
func (s *AuditStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Append inserts an audit event
func (s *AuditStore) Append(ctx context.Context, e audit.Event) error {
	meta, err := marshalNullable(e.Metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_events
		(id, actor, actor_type, action, target_type, target_id, outcome, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ID, e.Actor, e.ActorType, e.Action, e.TargetType, e.TargetID, e.Outcome, meta, e.Timestamp)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// List returns matching events, newest first
func (s *AuditStore) List(ctx context.Context, f audit.Filter) ([]audit.Event, error) {
	query := `SELECT id, actor, actor_type, action, target_type, target_id, outcome, metadata, occurred_at
		FROM audit_events WHERE TRUE`
	var args []any
	if f.Actor != "" {
		args = append(args, f.Actor)
		query += fmt.Sprintf(` AND actor = $%d`, len(args))
	}
	query += ` ORDER BY seq DESC`
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(` LIMIT $%d`, len(args))
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	out := make([]audit.Event, 0)
	for rows.Next() {
		var e audit.Event
		var meta []byte
		if err := rows.Scan(&e.ID, &e.Actor, &e.ActorType, &e.Action, &e.TargetType, &e.TargetID, &e.Outcome, &meta, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		e.Timestamp = e.Timestamp.UTC()
		if meta != nil {
			if err := json.Unmarshal(meta, &e.Metadata); err != nil {
				return nil, fmt.Errorf("decode audit metadata: %w", err)
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
