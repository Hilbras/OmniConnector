# ADR-002: Kernel Runtime Runs in-Process for Trusted Packages

## Status
Accepted (with caveats)

## Context

The initial release needs to execute operations quickly and reliably. Package
isolation adds complexity and latency. However, untrusted third-party packages
must not be allowed to compromise the kernel.

## Decision

- **Trusted first-party integrations** (GitHub, Vercel, SSH, SFTP, etc.) execute
  in-process within the Kernel Runtime.
- The `executionTarget` field in the manifest indicates where an operation
  must run: `cloud` (kernel backend), `local` (Local Agent), or `any`.
- Untrusted packages are **not** supported in the initial release.
- The contract is designed so that future isolation does not require replacing
  the public API.

## Consequences

- Initial integrations are fast and simple.
- Third-party package execution is deferred until isolation is mature.
- The `executionTarget` field provides a clear path to isolated workers.

## Alternatives Considered

1. **All packages in isolated workers** — Rejected for initial release due to
   complexity and latency.
2. **No execution model** — Rejected because security requires explicit
   execution target declarations.