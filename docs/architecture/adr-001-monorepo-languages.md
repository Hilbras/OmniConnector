# ADR-001: Monorepo with Go for Kernel and TypeScript for SDK

## Status
Accepted

## Context

OmniConnector requires a Kernel Runtime, SDK Package, Desktop App, and Web
Dashboard. The Kernel Runtime needs to manage packages, execute operations, and
coordinate with a Local Agent — all performance-sensitive tasks. The SDK
Package needs to support integrations that may be written by external
developers and consumed by JavaScript/TypeScript applications.

## Decision

- Use a **monorepo** for shared evolution of Kernel, SDK, and applications.
- Write the **Kernel Runtime and Local Agent in Go** (performance, concurrency,
  easy distribution).
- Write the **SDK Package and integrations in TypeScript** for client-side and
  server-side use.
- Use a **Go workspace** (`go.work`) to manage Go packages within the monorepo.
- Use **pnpm/npm** for TypeScript packages.

## Consequences

- The Kernel and SDK share a documented contract but are implemented in
  different languages. This boundary must be explicit and versioned.
- Go and TypeScript developers need separate toolchains, but each can evolve
  independently.
- The Kernel Runtime can run as a standalone binary without a Node.js runtime.

## Alternatives Considered

1. **All-Go** — Rejected because external integrations and applications are
   primarily JavaScript/TypeScript ecosystems.
2. **All-TypeScript** — Rejected because Go provides better performance for
   the operation dispatcher and easier binary distribution for the Local Agent.
3. **Polyglot with multiple repos** — Rejected because early evolution benefits
   from shared versioning and coordinated releases.