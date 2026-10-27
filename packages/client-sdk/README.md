# @hilbras/omniconnector

TypeScript client for the [OmniConnector](https://github.com/Hilbras/OmniConnector) Kernel API.

OmniConnector is a unified connection and integration layer: it owns credentials, permissions, and execution for services like GitHub, Vercel, SSH, and SFTP, so your application runs **operations** without ever handling the underlying secrets.

> **Status: 0.1 — early preview.** The Kernel API covers integration discovery, operations, and audit events. Connections, approvals, and user authentication arrive in upcoming releases; see the [project status](https://github.com/Hilbras/OmniConnector/blob/main/docs/STATUS.md). Expect breaking changes before 1.0.

## Install

```bash
npm install @hilbras/omniconnector
```

Requires Node.js 20.3+ (or any runtime with `fetch` and `AbortSignal.any`). No runtime dependencies.

## Usage

```ts
import { OmniConnectorClient, OmniConnectorError } from "@hilbras/omniconnector";

const api = new OmniConnectorClient({
  baseUrl: "https://connect.example.com",
  token: process.env.OMNICONNECTOR_TOKEN!,
});

// What can this kernel do?
const integrations = await api.integrations.list();
const github = await api.integrations.get("github");

// Run an operation
const op = await api.operations.create(
  { integration: "test-integration", operationId: "echo", input: { message: "hi" } },
  { idempotencyKey: crypto.randomUUID() },
);

if (op.state === "succeeded") {
  console.log(op.result);
} else {
  console.error(op.state, op.error?.category, op.error?.message);
}
```

### Rejected requests vs failed operations

The client distinguishes two kinds of failure:

- **The request is rejected** (bad input, missing permission, unknown integration, network error): the call **throws** an `OmniConnectorError` with `status`, `code`, `category`, `retryable`, and `requestId`.
- **The operation runs and fails**: the call **resolves** with an `Operation` whose `state` is `failed`, `timed_out`, `cancelled`, or `unknown_outcome`, and whose `error` explains why.

```ts
try {
  await api.operations.create({ integration: "github", operationId: "repository.create_branch", input });
} catch (err) {
  if (err instanceof OmniConnectorError && err.category === "permission_denied") {
    // ask for access…
  }
  console.error(`failed (request ${(err as OmniConnectorError).requestId})`);
}
```

### Retries and `unknown_outcome`

The client **never retries on its own**. When an operation with side effects times out or is interrupted, OmniConnector cannot know whether it took effect and reports `unknown_outcome` with `retryable: false`. Blindly retrying could, for example, deploy twice.

To retry safely, send an `idempotencyKey`: repeating a request with the same key returns the original operation instead of executing again.

### Cancellation and waiting

```ts
await api.operations.cancel(op.id);                       // state → cancel_requested
const final = await api.operations.wait(op.id, { timeoutMs: 30_000 });
```

### Audit events

```ts
const events = await api.auditEvents.list({ limit: 20 }); // requires audit.read
```

## API

| Method | Endpoint |
|---|---|
| `health()` / `ready()` | `GET /health`, `GET /ready` |
| `integrations.list()` | `GET /v1/integrations` |
| `integrations.get(name)` | `GET /v1/integrations/{name}` |
| `integrations.capabilities(name)` | `GET /v1/integrations/{name}/capabilities` |
| `operations.create(req, { idempotencyKey })` | `POST /v1/operations` |
| `operations.get(id)` / `operations.list({ limit })` | `GET /v1/operations[/{id}]` |
| `operations.cancel(id)` | `POST /v1/operations/{id}/cancel` |
| `operations.wait(id)` | polls `GET /v1/operations/{id}` |
| `auditEvents.list({ limit })` | `GET /v1/audit-events` |

Every method accepts `requestId` (sent as `X-Request-ID` for log correlation) and an `AbortSignal`.

## License

MIT © Hilbras
