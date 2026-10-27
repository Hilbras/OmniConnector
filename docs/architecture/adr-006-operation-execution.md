# ADR-006: Operation Execution Model

## Status
Accepted

## Context

Operations must be tracked, authorized, and executed reliably. The execution
engine must handle timeouts, cancellations, retries, and idempotency.

## Decision

- Use an **explicit state machine** for operations (see Operation States in
  the Integration Contract).
- Operations are **synchronous** for quick operations and **asynchronous** for
  long-running operations (file transfers, deployments).
- **Retries are operation-aware**: read-only operations can be retried safely;
  write operations require reconciliation before retry.
- **Idempotency keys** are supported for safe retries.
- **UnknownOutcome** is a terminal state when an external request may have
  succeeded but the response was lost.

## Consequences

- The state machine adds complexity but provides clear semantics for clients.
- UnknownOutcome prevents blind retries that could cause duplicate side effects.
- Operation metadata is persisted for audit and reconciliation.

## Alternatives Considered

1. **Fire-and-forget** — Rejected because operations must be trackable and
   auditable.
2. **Always asynchronous** — Rejected because quick operations (e.g., list
   repositories) should be synchronous for better UX.
3. **No unknown outcome** — Rejected because lost responses are inevitable in
   distributed systems.