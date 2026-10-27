# ADR-008: SDK Package Language

## Status
Accepted

## Context

The SDK Package must be consumable by both the Kernel Runtime (Go) and
external applications (JavaScript/TypeScript). It must define stable contracts
that integrations implement.

## Decision

- Write the **SDK Package in TypeScript** for client-side and server-side use.
- Write **Go bindings** in the `shared-contracts` package for the Kernel
  Runtime.
- The contract is defined in **both languages** and must be kept in sync.
- Integrations can be written in **either Go or TypeScript**, depending on
  their execution target.

## Consequences

- TypeScript integrations are easier for external developers to adopt.
- Go integrations are easier for performance-sensitive or local-execution
  integrations (SSH, SFTP).
- The contract must be maintained in two languages, which adds overhead.

## Alternatives Considered

1. **All-Go SDK** — Rejected because external developers primarily use
   JavaScript/TypeScript.
2. **All-TypeScript SDK** — Rejected because the Kernel Runtime is in Go and
   needs native bindings.
3. **WebAssembly** — Rejected for initial release due to complexity and
   immaturity in this context.