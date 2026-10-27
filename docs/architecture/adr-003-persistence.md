# ADR-003: PostgreSQL for Persistence

## Status
Accepted

## Context

OmniConnector needs to persist users, connections, credentials, operations,
audit events, and agent sessions. The data model includes relational data
with foreign keys, transactional guarantees, and audit trails.

## Decision

- Use **PostgreSQL** as the primary database.
- Store **encrypted credential references** in PostgreSQL; actual secret
  material is stored in a separate encrypted store (e.g., encrypted files,
  HashiCorp Vault, or OS keychain).
- Use **JSONB** columns for flexible configuration and operation metadata.
- Use **explicit foreign keys** and lifecycle rules.

## Consequences

- PostgreSQL provides strong consistency and relational integrity.
- Credential separation adds a layer of complexity but improves security.
- Redis, message brokers, and other infrastructure are deferred until
  justified by measured workload.

## Alternatives Considered

1. **SQLite** — Rejected for production due to lack of concurrent access and
   advanced features.
2. **NoSQL** — Rejected because relational data (users, connections,
   operations) benefits from foreign keys and transactions.
3. **Multiple databases** — Rejected for initial release to avoid operational
   complexity.