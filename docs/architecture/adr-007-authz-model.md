# ADR-007: Authentication and Authorization Model

## Status
Accepted

## Context

OmniConnector must support multiple authentication methods (OAuth, API tokens,
SSH keys, etc.) and enforce least-privilege authorization for every operation.

## Decision

- Implement **user authentication** (OIDC-compatible).
- Implement **application identities** for external clients.
- Implement **device identities** for Local Agents.
- Use **capability-driven authorization**: every operation is authorized
  against the caller, connection, and requested capability.
- Use **resource-level permissions**: e.g., "repo.read" vs "repo.admin".
- Use **approval policies** for sensitive operations (delete files, modify DNS,
  etc.).
- Use **scoped access tokens** for external applications.

## Consequences

- Authorization is checked at every operation execution, not just at the API
  gateway.
- Approval policies add latency to sensitive operations but improve safety.
- External applications receive only explicitly scoped access.

## Alternatives Considered

1. **Single global permission** — Rejected because least privilege is a core
   requirement.
2. **Role-based only** — Rejected because resource-level granularity is needed.
3. **No approval policies** — Rejected because destructive operations require
   explicit confirmation.