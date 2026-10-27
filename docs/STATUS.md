# OmniConnector — Status

Progress against [`PLAN.md`](PLAN.md). Update this file whenever a phase task lands.

_Last updated: 2026-10-27 (Phase 1 complete)_

## Phase overview

| # | Phase | Status |
|---|---|---|
| 0 | Product Definition & Architecture | 🟡 Partial — ADRs 001–010, integration contract spec, initial schema exist. Missing: PRD, threat model, OpenAPI spec, dev/testing standards |
| 1 | Kernel Foundation | ✅ Complete |
| 2 | Authentication & Connections | ⚪ Not started (credential AES-GCM store and permission engine exist as building blocks) |
| 3 | SSH & SFTP | ⚪ Stub manifests only; `Execute` returns `not_supported` |
| 4 | GitHub & Vercel | ⚪ Stub manifests + partial HTTP clients; `Execute` returns `not_supported` |
| 5 | Web Dashboard | ⚪ Not started |
| 6 | Desktop Application | ⚪ Not started |
| 7 | Local Agent | ⚪ Not started |
| 8 | Additional Integrations | ⚪ Not started |
| 9 | Public API & Client SDK | ⚪ Not started |
| 10 | Production Readiness | ⚪ Not started |

## Phase 1 — Kernel Foundation

**Exit criterion:** _a test integration can register, expose capabilities, receive a valid operation, return a normalized result, and report failures correctly._
**Met** — proven by `TestPhase1ExitCriteria` (`services/kernel-api/internal/handlers/handlers_test.go`) over HTTP, by `TestKernelOnPostgres` against a real database, and by a manual run of the docker-compose stack (operation and audit event survived an API restart; `/ready` returned 503 with the database stopped).

| Task | Status | Where |
|---|---|---|
| Repository and build system | ✅ | `go.work`, `Makefile` (iterates every workspace module) |
| Configuration management | ✅ | `packages/kernel-runtime/config` — env-driven, validated |
| Logging and request correlation | ✅ | `logger` (slog JSON); `X-Request-ID` propagated into operation records, audit metadata, and error envelopes |
| Package registration + lifecycle | ✅ | `kernel/kernel.go` — `Initialize` on register, `Shutdown` on kernel stop, per-package runtime context |
| Manifest validation | ✅ | `kernel/manifest.go` — version compatibility, identifiers, capability ↔ operation consistency, retry policy sanity, input schema keywords |
| Capability discovery | ✅ | Manifest is the source of truth; `GET /v1/integrations/{id}/capabilities` |
| Common error handling | ✅ | `contracts.Error` implements `error`; category → HTTP status mapping; raw package errors are never returned to clients |
| Operation dispatch | ✅ | `kernel/dispatch.go` — authn → authz (`operation.execute` + capability perms) → input schema → execution target → idempotency → bounded concurrency → timeout → normalize → persist → audit |
| Operation state model | ✅ | Timeouts/cancels/panics on non-idempotent ops → `unknown_outcome` (non-retryable) |
| Health endpoints | ✅ | `GET /health`, `GET /ready` |
| Initial persistence | ✅ | `pgstore` — PostgreSQL operation and audit stores, embedded versioned migrations (`pgstore/migrations`) applied at startup under an advisory lock. Audit rows are immutable (trigger). The idempotency unique index makes concurrent retries execute once. `storetest` is a shared suite both memory and PostgreSQL stores must pass |
| Deployment | ✅ | `deployment/Dockerfile.kernel-api` (distroless, non-root), `deployment/docker-compose.yml` (API + PostgreSQL); `/ready` pings the database |
| Unit tests and CI | ✅ | Kernel, config, API, stores, and a connector conformance suite (`tests/contract`); CI runs gofmt, vet, `-race` tests across all modules with a PostgreSQL service, and builds the image |

### Interim decisions (revisit in Phase 2)

- **Dev bearer token.** Until user/app authentication exists, the API accepts one static token (`KERNEL_DEV_TOKEN`, ≥32 chars) mapped to `dev-user` with `KERNEL_DEV_PERMISSIONS`. Unset → every authenticated route returns 401 (closed by default).
- **Synchronous execution.** `POST /v1/operations` runs to completion and returns the record (201 even if the operation itself failed). The operation is detached from the HTTP request context, so a dropped client doesn't abort a write; cancellation is explicit. Async dispatch for long operations (ADR-006) is not built yet.
- **Local execution target** is rejected with `not_supported` until the Local Agent exists.
- **Connections and approvals** routes return `501 not_implemented` rather than placeholder success.
- **Credential key**: `KERNEL_CREDENTIAL_KEY` (base64, 32 bytes). Unset → ephemeral random key with a warning. The previous hardcoded key was removed.
- **Persistence is optional.** `KERNEL_DATABASE_DSN` unset → in-memory stores with a startup warning. `KERNEL_DATABASE_AUTO_MIGRATE=false` disables startup migrations.
- **Old `deployment/schema.sql`** moved to `docs/architecture/draft-schema.sql` as a design reference. It was never applied and conflicts with the current model (stored raw input, mandatory connection FK, UUID-typed caller/target IDs).
- **Interrupted operations.** An operation that is `queued`/`running` when the process dies stays in that state in the database. The `operations_active` index exists for a reconciliation sweep, which is not built yet (Phase 10 lists interrupted-operation recovery; it may be pulled earlier).

## Next up

1. Phase 0 gaps: threat model and an OpenAPI document for the endpoints that now exist.
2. Startup reconciliation: mark operations left `queued`/`running` by a crash as `unknown_outcome` (or `cancelled` if never started).
3. Phase 2: user/app authentication replacing the dev token, connection records with ownership, credential references, connection test via `ValidateConnection`.

## Running locally

```bash
make test-race      # all modules, race detector (PostgreSQL tests skip)
make test-pg        # PostgreSQL store tests in a throwaway container
make run-dev        # API on :8080, in-memory stores, test integration, admin dev token
make docker-up      # API + PostgreSQL via docker compose (same token)
curl -H "Authorization: Bearer dev-token-change-me-0123456789abcdef" \
     -H "Content-Type: application/json" \
     -d '{"integration":"test-integration","operationId":"echo","input":{"message":"hi"}}' \
     localhost:8080/v1/operations
```
