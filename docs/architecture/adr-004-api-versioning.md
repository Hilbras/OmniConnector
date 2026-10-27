# ADR-004: API Versioning Policy

## Status
Accepted

## Context

The API must evolve without breaking existing clients. External applications
and internal clients (Desktop, Web) depend on stable API contracts.

## Decision

- Use **URL-based versioning** with `/v1/` prefix.
- Maintain **backward compatibility** for at least one minor version behind.
- Use **OpenAPI 3** for API documentation.
- Use **semantic versioning** for API releases.
- Deprecation must be announced in advance; removed endpoints must follow a
  documented sunset policy.

## Consequences

- New features are added as additive changes within the same version.
- Breaking changes require a new major version (`/v2/`).
- OpenAPI documentation is the source of truth for the API contract.

## Alternatives Considered

1. **No versioning** — Rejected because breaking changes are inevitable.
2. **Header-based versioning** — Rejected because it's harder to discover and
   document.
3. **Major version only** — Rejected because it discourages incremental
   improvements.