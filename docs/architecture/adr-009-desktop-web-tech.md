# ADR-009: Desktop and Web Technology

## Status
Accepted

## Context

The Desktop App and Web Dashboard must provide consistent functionality across
platforms. The Desktop App needs native capabilities (filesystem access, SSH
key management, SFTP performance).

## Decision

- Use **Tauri 2 + React + TypeScript** for the Desktop App.
- Use **React + TypeScript** for the Web Dashboard.
- Share UI components and API clients between Desktop and Web where possible.
- Use **native Tauri plugins** for filesystem access, SSH key management, and
  SFTP performance where needed.

## Consequences

- Tauri provides a lightweight alternative to Electron with smaller binary size.
- Shared code reduces duplication between Desktop and Web.
- Native capabilities are available through Tauri plugins.

## Alternatives Considered

1. **Electron** — Rejected because of larger binary size and memory usage.
2. **Native (Swift, GTK, etc.)** — Rejected because of increased development
   overhead and cross-platform challenges.
3. **Web-only** — Rejected because the Desktop App needs local filesystem and
   SSH key access.