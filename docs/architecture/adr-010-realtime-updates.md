# ADR-010: Realtime Updates

## Status
Accepted

## Context

Users need real-time updates for operation progress, transfer status, and
agent connectivity. The Web Dashboard and Desktop App must receive these
updates without polling.

## Decision

- Use **Server-Sent Events (SSE)** for one-way server-to-client updates.
- Use **WebSockets** for bidirectional communication where needed (e.g., Local
  Agent task delivery).
- Use **PostgreSQL LISTEN/NOTIFY** for in-process event distribution (no Redis
  required for initial release).

## Consequences

- SSE is simpler than WebSockets for one-way updates.
- WebSockets are needed for Local Agent task delivery.
- PostgreSQL LISTEN/NOTIFY avoids external message broker dependencies.

## Alternatives Considered

1. **Polling only** — Rejected because it's inefficient and provides poor UX.
2. **Redis + WebSocket** — Rejected for initial release due to added
   operational complexity.
3. **WebSocket only** — Rejected because SSE is simpler for one-way updates.