# OmniConnector — Complete Product, Architecture & Development Plan

> **Status:** Governing goal for the project. Progress against it is tracked in
> [`docs/STATUS.md`](STATUS.md). Changes to scope belong here; changes to
> decisions belong in an ADR under `docs/architecture/`.

**Product:** OmniConnector
**Organization:** Hilbras
**Product Category:** Unified Connections & Integration Platform
**Platforms:** Desktop Application + Web Dashboard
**Core Architecture:** Kernel / API + Kernel Runtime + SDK Package
**Primary Goal:** Connect services, manage authentication and credentials, execute operations, and expose integrations to other applications through standardized interfaces.

---

## 1. Product Vision

OmniConnector is a unified platform for connecting, managing, and operating external services, development platforms, servers, and infrastructure from a single environment. Instead of integrating GitHub, Vercel, SSH servers, databases, and other services separately in every application, OmniConnector provides a centralized integration layer that handles connections, authentication, capabilities, permissions, and operation execution.

OmniConnector consists of:

- A Desktop Application for managing local and remote connections.
- A Web Dashboard for managing accounts, connections, projects, and supported operations.
- A Kernel/API layer that connects applications to the internal runtime.
- A Kernel Runtime responsible for managing packages and coordinating operations.
- An SDK Package containing service integrations (GitHub, Vercel, SSH, SFTP, …).
- A Local Agent for operations that require access to the user's computer or local credentials.
- A public API and client SDK for other Hilbras products and external applications.

### 1.1 Core Objectives

1. Unified interface for managing multiple service connections.
2. Centralized service-specific authentication and connection management.
3. File transfers and remote server management.
4. Source control and deployment platform integration.
5. Consistent, permission-controlled operation execution.
6. External applications use approved connections without handling credentials.
7. Extensible integrations through a standardized package contract.
8. Consistent functionality across Desktop and Web.
9. Reliable operation tracking, error handling, and audit records.
10. Foundation for future integrations, automation, and agent-based workflows.

### 1.2 Non-Goals for the Initial Release

- Replacing every existing cloud management platform.
- Supporting every provider before the integration architecture is stable.
- Unrestricted shell execution for AI agents.
- A public integration marketplace before the package system is mature.
- Unnecessary microservices.
- Synchronizing all local credentials to cloud infrastructure.
- A complete workflow automation platform before core operations are reliable.

---

## 2. Product Architecture

Modular architecture with clear boundaries between application interfaces, API handling, the kernel runtime, service integrations, execution environments, and persistent storage.

### 2.1 High-Level Architecture

```text
OmniConnector
├── Applications        Desktop App · Web Dashboard · CLI · External Applications
├── Kernel / API        API Gateway · Authentication · Authorization · Request Validation
│                       Operation Routing · Response Handling
├── Kernel Runtime      Package Registry · Package Lifecycle Manager · Operation Dispatcher
│                       Capability Registry · Policy Engine · Event System · Job Coordinator
├── SDK Package         Shared Integration Interfaces · Authentication Adapters · Connection Mgmt
│                       GitHub · GitLab · Vercel · SSH · SFTP · Cloud · Database · Additional
├── Execution Layer     Cloud Execution Workers · Local Agent · Isolated Execution Environments
└── Shared Infra        PostgreSQL · Encrypted Credential Storage · Audit Events · Job State
                        Observability
```

This is a logical architecture. In the first release, the API and Kernel Runtime may run inside the same backend application while maintaining separate internal modules.

### 2.2 Kernel / API

Interface between applications and the Kernel Runtime. Responsibilities: receive requests from Desktop, Web, CLI, and external clients; authenticate callers; validate request schemas; check permissions and operation policies; resolve connection and package identifiers; route to the appropriate package and execution environment; return normalized responses; stream progress where supported; enforce rate and request-size limits; record audit events.

The API must not expose internal credentials or allow clients to bypass authorization by calling a package directly.

### 2.3 Kernel Runtime

Manages the internal lifecycle and execution of integrations: discover/register packages; validate manifests and compatibility; manage versions and lifecycle; register capabilities; resolve operations to packages; coordinate execution and cancellation; apply permission and approval policies; normalize results and errors; manage events and job state; prevent incompatible packages from loading; support future package isolation.

The Kernel must not contain service-specific business logic (e.g. GitHub repository operations belong in the GitHub integration).

### 2.4 SDK Package

First-class package containing the integration framework and service-specific connectors: shared interfaces, provider auth flows, credential/connection adapters, provider API clients, operations, capability definitions, error normalization, connection validation, rate-limit handling, configuration schemas, compatibility/version metadata. Exposes stable interfaces to the Kernel Runtime. Other Hilbras products consume capabilities through the public API or client SDK, never connector internals.

### 2.5 Local Agent

Optional component on the user's computer for operations that cannot safely or practically execute in the cloud: local files, locally stored SSH keys, private network resources, local dev environments, approved local tasks, local tools when explicitly enabled. Must authenticate to OmniConnector, maintain an explicit allow-list of capabilities, and reject unauthorized remote commands. Installing it must not give the Web Dashboard unrestricted access to the computer.

---

## 3. SDK Package Design

### 3.1 Proposed Internal Structure

```text
sdk/
├── core/          integration-contracts · authentication-interfaces · connection-interfaces
│                  operation-interfaces · capability-definitions · error-models · configuration-schemas
├── auth/          oauth2 · api-token · ssh-key · username-password · certificate-auth
├── integrations/  github · gitlab · bitbucket · vercel · netlify · cloudflare · ssh · sftp · ftp
│                  aws · digitalocean · docker · kubernetes · postgresql · mysql
├── shared/        http-client · retry-policy · pagination · rate-limit-handler · telemetry · validation
├── testing/       connector-conformance-tests · mock-providers · integration-test-utilities
└── index
```

Conceptual; the final layout follows the chosen language and build system.

### 3.2 Standard Integration Contract

Every integration implements: **Manifest**, **Configuration** validation, **Authentication** (start/complete/validate/refresh/revoke where supported), **Connection Validation**, **Capabilities** (operations + required permissions), **Execution** of validated input, **Error Handling** into normalized categories, **Health**, **Lifecycle** (init/shutdown/cleanup), **Audit Metadata** without secrets. Capabilities are optional per provider — the contract must not pretend every integration behaves identically.

### 3.3 Package Manifest

Package identifier, display name, version, package API version, supported runtime versions, required permissions, supported auth methods, supported operations, required config fields, network requirements, execution environment requirements, dependencies, security metadata. Installation and execution validate the manifest before accepting the package.

### 3.4 Package Execution Model

Initially, trusted integrations may run in-process — but in-process packages share the runtime's security boundary and are not "isolated" just because they implement the contract. Packages that run arbitrary code, third-party extensions, or need elevated access should eventually use isolated workers; the public contract must allow that transition without redesign.

---

## 4. Initial Integration Roadmap

### Phase A: Core Development Integrations

- **GitHub** — OAuth, GitHub App, fine-grained tokens; secure storage/revocation. Capabilities: authorized account, list repos, repo metadata, browse/read files, create branches, create/update files, create/inspect PRs, issues, workflow/deployment info. Respect repo permissions, pagination, rate limits, revoked credentials; never leak private content.
- **Vercel** — OAuth or user tokens. Teams/projects, deployments, status, logs, trigger deployments, domains and env metadata. Sensitive env values never exposed by default.
- **SSH** — key auth, password where explicitly supported, host-key verification and trusted-host management. Connectivity test, approved system info, explicitly authorized commands, directory inspection, output + exit status, timeouts/cancellation. Protect keys, enforce command/destination policies, no credentials in logs.
- **SFTP** — list, metadata, download, upload, rename/move, mkdir, delete when authorized, resume, progress, interruption handling. Its own module even if it shares SSH transport.
- **Cloudflare** — accounts/zones, DNS read, DNS create/update/delete with explicit authorization, platform resources, token-permission validation. Confirmation for production-disrupting DNS changes.
- **PostgreSQL** — validate connections, server/db metadata, schemas/tables, read-only queries by default, bounded structured results. Writes need extra privilege + policy; timeouts and resource limits.

### Phase B (after Phase A passes conformance + security tests)

GitLab, Bitbucket, Netlify, FTP/FTPS, DigitalOcean, AWS, Docker, MySQL, Amazon S3, Cloudflare R2, GitHub Actions, GitLab CI, Kubernetes.

### Phase C: Extended Ecosystem

Google Cloud, Azure, MongoDB, Redis, Sentry, Grafana, Datadog, Linear, Jira, Slack, Discord, Telegram, Route 53, Jenkins, npm/GitHub Packages. Each evaluated on real API capability, auth constraints, demand, maintenance cost, and security.

---

## 5. Authentication and Credential Management

SDK handles provider-specific flows; shared infrastructure provides secure storage, access control, lifecycle, and auditing.

- **Methods:** OAuth 2.0/provider OAuth, API tokens, SSH keys, username/password where required, client certificates, cloud identity, future adapters. No single forced mechanism.
- **Lifecycle:** secure creation/import, validation, encrypted storage, restricted access, expiration metadata, refresh, revocation, rotation, deletion, audit.
- **Storage:** encryption at rest with secure key management. Distinguish cloud-managed, local-only, temporary, single-operation, application-level, and user-level credentials. Secrets are never returned to the frontend after creation (except explicit one-time display). Local-only credentials stay on device by default.

---

## 6. Connection Management

- **Lifecycle:** create → configure → authenticate → validate → activate → execute → monitor health → refresh/reconnect → disable → revoke/delete.
- **Metadata:** ID, owner and access scope, provider, display name, config metadata, credential references, execution target, granted permissions, status, timestamps, last successful validation, last operation status. Never store passwords/tokens in ordinary connection records.
- **Types:** account, repository, deployment platform, server, file transfer, database, cloud infrastructure, local resource. One authenticated account can serve many resources without duplicating credentials.

---

## 7. Operation Execution Engine

### 7.1 Lifecycle

Request Received → Validate Input → Authenticate Caller → Authorize Operation → Resolve Connection → Resolve Integration → Select Execution Target → Apply Approval Policy → Execute → Normalize Result → Persist Status and Audit Event → Return Result.

### 7.2 States

Pending, Validating, AwaitingApproval, Queued, Running, Succeeded, Failed, CancelRequested, Cancelled, TimedOut, **UnknownOutcome** (external request may have succeeded but the response was lost — blind retry could duplicate side effects).

### 7.3 Metadata

Operation ID, caller identity, connection ID, package + operation ID, validated input metadata, execution target, authorization decision, approval state, start/finish timestamps, final status, result reference, error category, retry/timeout metadata. Sensitive inputs/outputs redacted from ordinary logs.

### 7.4 Requirements

Timeouts, cancellation, bounded concurrency, rate limits, retry policies for safe operations, idempotency keys, structured errors, progress events, output size limits, audit records. Retries are operation-aware: reads can usually retry; writes may need reconciliation first.

---

## 8. Kernel/API Design

### 8.1 API Groups

`/v1/auth/*`, `/v1/users/*`, `/v1/connections/*`, `/v1/integrations/*`, `/v1/capabilities/*`, `/v1/operations/*`, `/v1/approvals/*`, `/v1/agents/*`, `/v1/events/*`, `/v1/audit-events/*`

### 8.2 Example Endpoints

```http
GET    /v1/integrations
GET    /v1/integrations/{integrationId}
GET    /v1/integrations/{integrationId}/capabilities
GET    /v1/connections
POST   /v1/connections
GET    /v1/connections/{connectionId}
PATCH  /v1/connections/{connectionId}
DELETE /v1/connections/{connectionId}
POST   /v1/connections/{connectionId}/test
POST   /v1/operations
GET    /v1/operations/{operationId}
POST   /v1/operations/{operationId}/cancel
GET    /v1/approvals
POST   /v1/approvals/{approvalId}/approve
POST   /v1/approvals/{approvalId}/reject
GET    /v1/audit-events
```

Proposed, not final; the contract is versioned and documented before applications depend on it.

### 8.3 Requirements

Consistent schemas, strong authentication, resource-level authorization, input validation, pagination, rate limiting, structured errors, request and correlation IDs, idempotency, versioning, OpenAPI docs, cross-account protection.

### 8.4 External Client Access

Application registration, scoped tokens, expiration/revocation, per-app permissions, connection-level policies, usage limits, audit. An external app never automatically gains access to all of a user's connections.

---

## 9. Desktop Application

Primary interface for local and remote infrastructure.

- **Dashboard:** connection summary, recent operations/errors, active transfers, service status, quick actions.
- **Connections:** add, configure auth, test, edit, inspect permissions, disable/remove.
- **Server Explorer:** browse, upload/download, rename/move, mkdir, delete with confirmation, metadata, transfer monitoring.
- **Service Explorer:** GitHub repos, Vercel projects, deployment status, other capabilities.
- **Operations:** active/completed jobs, results, errors, cancellation, reconcile uncertain outcomes.
- **Settings:** Local Agent, credential storage, security, devices, notifications, diagnostics.

Suggested: Tauri 2 + React + TypeScript, weighed against SFTP throughput, local key management, filesystem access, and deployment model.

---

## 10. Web Dashboard

Screens: dashboard, connections, integrations, connection configuration, repo/project explorers, deployment management, operation history, approval queue, audit log, account/app settings, connected Local Agents.

Local resources are reached only through an authenticated Local Agent that registers capabilities, receives only authorized tasks, enforces local policy, reports status, rejects expired/unauthorized requests, and never accepts commands from unauthenticated clients. Cloud vs local execution is an explicit, policy-governed choice.

---

## 11. Local Agent Architecture

- **Responsibilities:** secure registration/auth, local credential access, filesystem ops, local SSH/SFTP, private network connectivity, approved local commands, operation reporting, health monitoring, capability discovery.
- **Security:** no unauthenticated remote command interface, device revocation, short-lived credentials, explicit capability grants, local execution policies, strict validation, no secret leakage in logs, replay/expiry protection, per-user isolation where supported. It must never become a general-purpose remote administration backdoor.

---

## 12. Desktop and Web Synchronization

- **Synchronizable:** connection names/metadata, provider IDs, project references, preferences, operation history, approval state, agent registration metadata.
- **Local-only by default:** private keys, local-only credentials, unnecessary sensitive paths, temporary auth material, unencrypted secrets. Users may opt into cloud-managed credentials where appropriate.

---

## 13. Security Architecture

- **Identity & authorization:** user auth, sessions, application identities, device identities, connection ownership, role/resource permissions, operation-level authorization, token revocation, admin audit trails.
- **Least privilege:** repo read ≠ repo admin; DNS read ≠ DNS write; DB browse ≠ modify; download ≠ delete; read-only connections reject writes.
- **Approval policies** for: deleting remote files, production DNS changes, production env config changes, unrestricted shell, destructive DB ops, installing/upgrading untrusted packages. Both user confirmation and app-level restrictions.
- **Package security:** pin/verify versions, validate manifests, dependency inventories, scanning, restricted permissions, audited updates, isolate untrusted integrations, signing before third-party distribution.
- **Audit & data protection:** actor, target, action, authorization decision, timestamp, outcome — never passwords, tokens, keys, or unrestricted sensitive file contents.

---

## 14. Data Model (PostgreSQL)

| Entity | Purpose |
|---|---|
| Users | User identities and account metadata |
| Applications | Registered clients and integrations |
| Devices | Registered Local Agents and device state |
| Integrations | Installed or supported integration definitions |
| PackageVersions | Package compatibility and version metadata |
| Connections | Configured service connections |
| Credentials | References to encrypted secret material |
| Capabilities | Supported operations and required permissions |
| Operations | Execution requests and state |
| OperationResults | Structured results or references to stored output |
| Approvals | Pending and completed approval decisions |
| AuditEvents | Security and operation audit records |
| AgentSessions | Local Agent connectivity and session metadata |

Rules: separate credentials from connection metadata; enforce ownership at the data access layer; explicit foreign keys and lifecycle rules; retention policies for history and audit; no large outputs in ordinary records; sensitive output only when necessary and access-controlled; audit logs are not a substitute for authorization.

---

## 15. Recommended Technology Stack

| Component | Starting Point |
|---|---|
| Desktop | Tauri 2 + React + TypeScript |
| Web | React + TypeScript |
| Backend | Go |
| Local Agent | Go |
| SDK | TypeScript for JS/TS clients; provider adapters around stable contracts |
| Database | PostgreSQL |
| Job Coordination | PostgreSQL-backed job state and controlled workers |
| Realtime | SSE; WebSockets where bidirectional is needed |
| API Docs | OpenAPI |
| Deployment | Docker |
| Testing | Unit, contract, integration, security, end-to-end |
| Observability | Structured logs, metrics, traces, correlation IDs |

If Go owns the runtime and TypeScript implements adapters, the boundary must be explicit and versioned. No Redis, message broker, or Kubernetes until measured need justifies it.

---

## 16. Development Phases

| # | Phase | Exit Criteria |
|---|---|---|
| 0 | Product Definition & Architecture — PRD, architecture doc, integration contract spec, threat model, API design, initial schema, dev/testing standards, ADRs | Architecture documented, package boundaries clear, no implementation decision rests on unresolved assumptions |
| 1 | Kernel Foundation — repo/build, config, logging + correlation, package registration, manifest validation, capability discovery, common errors, operation dispatch, health endpoints, initial persistence, unit tests + CI | A test integration can register, expose capabilities, receive a valid operation, return a normalized result, and report failures correctly |
| 2 | Authentication & Connections — user/app auth, connection records, credential refs, encrypted secret storage, auth adapter interfaces, connection validation, rotation/revocation, connection-level permissions, audit events, negative authz tests | Credentials never exposed through ordinary responses; unauthorized callers cannot access another user's connections; connection failures handled consistently |
| 3 | SSH & SFTP — host verification, key/password auth, timeouts, approved command execution, directory browsing, upload/download, progress + cancellation, file permissions, partial transfer recovery, controlled test servers | Users connect to test servers, browse and transfer files, recover from expected network failures, perform only authorized operations |
| 4 | GitHub & Vercel — provider auth, discovery, repo ops, project/deployment ops, pagination + rate limits, error normalization, capability permission checks, integration tests | Supported ops work end-to-end, permissions respected, revoked credentials handled, provider failures produce useful normalized errors |
| 5 | Web Dashboard — auth, dashboard, connections, integration discovery, config workflows, operation history, status views, approvals, secure API comms, a11y/responsive tests | Users manage cloud connections, run authorized ops, inspect results without bypassing Kernel policies |
| 6 | Desktop Application — app init, secure Kernel/API comms, Local Agent integration, server explorer, transfers, service explorers, local credentials, notifications, cross-platform packaging | Desktop behaves consistently with the API contract, local secrets protected, unsupported platform features clearly identified |
| 7 | Local Agent — device registration, short-lived auth, capability advertisement, secure task delivery, local policies, result reporting, revocation, reconnect, local audit, malicious-request tests | Agent accepts only authenticated, authorized tasks and cannot bypass local policies |
| 8 | Additional Integrations — Cloudflare, PostgreSQL, GitLab, Netlify, FTP/FTPS, DigitalOcean, Docker, MySQL, AWS, then demand-driven | Each passes the same contract, security, and compatibility checks as the initial integrations |
| 9 | Public API & Client SDK — finalized external contract, OpenAPI, scoped app tokens, app registration, expiry/revocation, client SDK, examples, compatibility guarantees, usage limits | An external app accesses explicitly authorized capabilities without direct access to secrets |
| 10 | Production Readiness — security review, dependency scanning, load/concurrency tests, interrupted-op recovery, migration/restore tests, backup, monitoring/alerting, incident response, deploy/rollback, E2E acceptance | Platform meets documented reliability, security, recovery, and operational requirements |

---

## 17. Testing Strategy

- **Unit:** config validation, manifest validation, permission evaluation, state transitions, credential lifecycle, error normalization, retry decisions.
- **Contract:** registration, capability declarations, I/O schemas, error format, auth interfaces, lifecycle, runtime compatibility.
- **Integration:** OAuth authorize/revoke, GitHub repo access, Vercel deployment inspection, SSH host verification, SFTP transfers, DB connectivity, rate limits, network interruptions.
- **Security:** cross-account access, credential leakage, invalid tokens, permission bypass, malicious packages, command injection, path traversal, SSRF, unauthorized agent tasks, replay/expired tasks.
- **End-to-end:** create connection → authenticate → validate → execute authorized op → track progress → inspect result → revoke → verify revoked access fails.

---

## 18. Repository Organization

Monorepo: `apps/{desktop,web}`, `services/{kernel-api,local-agent}`, `packages/{kernel-runtime,sdk,shared-contracts,client-sdk,ui}`, `integrations/{github,vercel,ssh,sftp,cloudflare,postgresql}`, `tests/{contract,integration,security,e2e}`, `docs/{architecture,api,integrations,security,development}`, `deployment/`, `scripts/`. Each language keeps its own appropriate build and test setup; don't force incompatible ecosystems into one package manager.

---

## 19. Operational Requirements

- **Reliability:** availability target, timeout policies, interrupted-task recovery, backup/restore, controlled migrations.
- **Performance:** API latency, bounded concurrency, transfer throughput, worker CPU/memory limits, pagination and output limits.
- **Observability:** structured logs, request/operation correlation IDs, failure and connection-health metrics, error classification, audit monitoring, critical alerts.
- **Deployment:** reproducible builds, env-specific config, secret management, health checks, rollback, dependency updates, staging, production checklist.

---

## 20. Initial Release Definition

**Required:** user auth, connection management, secure credential handling, Kernel/API, Kernel Runtime, SDK integration contract, GitHub, Vercel, SSH, SFTP, Web Dashboard, Desktop App, operation history, basic audit, scoped authorization, automated tests, API docs.

**Deferred:** public marketplace, arbitrary third-party package execution, large-scale workflow automation, broad Kubernetes management, extensive cloud provisioning, AI-controlled unrestricted shell, complex multi-tenant enterprise administration.

---

## 21. Success Criteria

- **Architecture:** new integrations added without modifying Kernel core logic; standardized manifests/capabilities; stable API contract.
- **Security:** no secrets in ordinary responses or logs; every operation authorized against caller, connection, and capability; revoked credentials/devices lose access; sensitive ops follow approval policy.
- **Functionality:** connect GitHub and Vercel; connect authorized SSH/SFTP servers; browse and transfer files; execute provider operations; inspect status and results.
- **Reliability:** interrupted ops represented accurately; unsafe retries prevented; errors normalized; integration tests validate real workflows.
- **Extensibility:** documented contract; validated compatibility; isolated execution without replacing the public API; external apps use capabilities through the client SDK.

---

## 22. Architectural Principles

1. **Kernel-first** — Kernel owns shared runtime behavior, not provider logic.
2. **SDK-centered integrations.**
3. **API-mediated access.**
4. **Capability-driven design.**
5. **Provider-specific authentication.**
6. **Least privilege.**
7. **Execution transparency** — every operation has an identifiable target, state, and outcome.
8. **Local and cloud separation.**
9. **Modular monolith first;** isolate where justified.
10. **Incremental integration growth.**
11. **Stable public interfaces.**
12. **Security before extensibility.**

## Build Order

**Kernel Foundation → SDK Contract → Authentication & Connections → SSH/SFTP → GitHub/Vercel → Web Dashboard → Desktop App → Local Agent → Additional Integrations → Public Client SDK → Production Hardening.**

The most important early milestone is not the number of integrations. It is proving that one integration can be registered, authenticated, authorized, executed, audited, and used through both the Kernel/API and the application interfaces without provider-specific logic inside the Kernel.
