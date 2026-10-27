-- Phase 1 persistence: operation records and audit events.
--
-- Identifiers are opaque prefixed strings (op_<uuid>, aud_<uuid>) generated
-- by the kernel, so they are TEXT, not UUID. Caller and connection IDs are
-- TEXT for the same reason; foreign keys to users and connections are added
-- by the Phase 2 migration that creates those tables.
--
-- Operation input is deliberately not stored: it may contain secrets.

CREATE TABLE operations (
    id                     TEXT PRIMARY KEY,
    request_id             TEXT NOT NULL DEFAULT '',
    integration            TEXT NOT NULL,
    operation_id           TEXT NOT NULL,
    capability             TEXT NOT NULL,
    connection_id          TEXT NOT NULL DEFAULT '',
    caller_user_id         TEXT NOT NULL DEFAULT '',
    caller_app_id          TEXT NOT NULL DEFAULT '',
    execution_target       TEXT NOT NULL,
    authorization_decision TEXT NOT NULL,
    idempotency_key        TEXT NOT NULL DEFAULT '',
    idempotent             BOOLEAN NOT NULL,
    timeout_seconds        INTEGER NOT NULL CHECK (timeout_seconds >= 0),
    state                  TEXT NOT NULL CHECK (state IN (
                               'pending', 'validating', 'awaiting_approval', 'queued', 'running',
                               'succeeded', 'failed', 'cancel_requested', 'cancelled',
                               'timed_out', 'unknown_outcome')),
    result                 JSONB,
    result_metadata        JSONB,
    error                  JSONB,
    created_at             TIMESTAMPTZ NOT NULL,
    started_at             TIMESTAMPTZ,
    finished_at            TIMESTAMPTZ,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT operations_has_caller CHECK (caller_user_id <> '' OR caller_app_id <> '')
);

-- Enforces one operation per caller and key, so concurrent retries cannot both execute
CREATE UNIQUE INDEX operations_idempotency_key
    ON operations (caller_user_id, caller_app_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE INDEX operations_caller_history ON operations (caller_user_id, caller_app_id, created_at DESC);
CREATE INDEX operations_history ON operations (created_at DESC);
-- Supports reconciliation of operations interrupted by a crash or restart
CREATE INDEX operations_active ON operations (state)
    WHERE state IN ('pending', 'validating', 'awaiting_approval', 'queued', 'running', 'cancel_requested');

CREATE TABLE audit_events (
    seq         BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    id          TEXT PRIMARY KEY,
    actor       TEXT NOT NULL,
    actor_type  TEXT NOT NULL,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    outcome     TEXT NOT NULL,
    metadata    JSONB,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX audit_events_actor ON audit_events (actor, seq DESC);
CREATE INDEX audit_events_occurred ON audit_events (occurred_at);

-- Audit events are append-only. Deletion stays possible for retention jobs;
-- rewriting history is not.
CREATE FUNCTION audit_events_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events rows are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_immutable();
