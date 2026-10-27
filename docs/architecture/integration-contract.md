# Integration Contract Specification

## Purpose

This specification defines the standardized contract that every OmniConnector
integration package must implement. It allows the Kernel Runtime to discover,
validate, authorize, execute, and audit integrations regardless of the
underlying service.

The contract is implemented in two layers:

1. **Go interfaces** — consumed directly by the Kernel Runtime and service
   packages written in Go.
2. **TypeScript interfaces** — consumed by the SDK Package and integrations
   written in TypeScript/JavaScript.

Both layers map to the same semantic contract described here.

## Package Manifest

Each integration must provide a manifest describing itself. The manifest is the
kernel's source of truth for compatibility, capabilities, and requirements.

```yaml
apiVersion: omni/v1
kind: IntegrationManifest
metadata:
  name: github
  displayName: GitHub
  version: 1.0.0
  packageApiVersion: v1
  description: GitHub integration for repository and deployment operations
spec:
  supportedRuntimeVersions: ["v1.0"]
  executionTarget: cloud   # cloud | local | any
  networkRequirements:
    outbound: ["api.github.com:443"]
    inbound: []
  dependencies: []
  authMethods:
    - oauth2
    - apiToken
    - githubApp
  configSchema:
    type: object
    properties:
      baseUrl:
        type: string
        description: API base URL (defaults to https://api.github.com)
    required: []
  capabilities:
    - name: repository.list
      permissions: ["repo.read"]
      inputSchema: {}
      outputSchema: {}
    - name: repository.read
      permissions: ["repo.read"]
      inputSchema:
        type: object
        properties:
          owner: { type: string }
          repo: { type: string }
          path: { type: string }
        required: ["owner", "repo", "path"]
      outputSchema:
        type: object
        properties:
          content: { type: string }
          encoding: { type: string }
  operations:
    - id: repositories.list
      capability: repository.list
      handler: ListRepositories
      timeoutSeconds: 30
      retryPolicy: safe
      idempotent: true
    - id: repository.read_file
      capability: repository.read
      handler: ReadRepositoryFile
      timeoutSeconds: 30
      retryPolicy: safe
      idempotent: true
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `apiVersion` | string | Manifest format version (omni/v1) |
| `metadata.name` | string | Unique integration identifier (kebab-case) |
| `metadata.displayName` | string | Human-readable name |
| `metadata.version` | string | Semantic version of the integration package |
| `metadata.packageApiVersion` | string | Contract API version this package targets |
| `spec.supportedRuntimeVersions` | array | Kernel Runtime versions this package is compatible with |
| `spec.executionTarget` | string | Where this integration executes (cloud, local, any) |
| `spec.networkRequirements` | object | Required network connectivity |
| `spec.dependencies` | array | Other integration packages required |
| `spec.authMethods` | array | Supported authentication methods |
| `spec.configSchema` | object | JSON Schema for configuration validation |
| `spec.capabilities` | array | Declared capabilities and permissions |
| `spec.operations` | array | Available operations |

## Capabilities

A capability is an abstract unit of functionality an integration can perform.
Capabilities declare required permissions so the kernel can evaluate authorization.

```go
type Capability struct {
    Name        string          // e.g. "repository.read"
    Permissions []string        // e.g. ["repo.read"]
    Description string
}
```

## Authentication Interface (Go)

```go
package contracts

// AuthMethod identifies an authentication flow
type AuthMethod string

const (
    AuthOAuth2       AuthMethod = "oauth2"
    AuthAPIToken     AuthMethod = "apiToken"
    AuthSSHKey       AuthMethod = "sshKey"
    AuthUsernamePass AuthMethod = "usernamePassword"
    AuthCertificate  AuthMethod = "certificate"
    AuthGitHubApp    AuthMethod = "githubApp"
)

// Authenticator is implemented by integrations to manage auth lifecycles
type Authenticator interface {
    // SupportedAuthMethods returns the auth methods this integration supports
    SupportedAuthMethods() []AuthMethod

    // StartAuth begins an authentication flow (e.g., redirects to OAuth)
    StartAuth(ctx context.Context, config AuthConfig) (*AuthChallenge, error)

    // CompleteAuth completes an authentication flow
    CompleteAuth(ctx context.Context, response AuthResponse) (*Credential, error)

    // ValidateCredential checks whether a credential is still valid
    ValidateCredential(ctx context.Context, cred *Credential) error

    // RefreshCredential refreshes an expired credential
    RefreshCredential(ctx context.Context, cred *Credential) (*Credential, error)

    // RevokeCredential revokes a credential
    RevokeCredential(ctx context.Context, cred *Credential) error
}

type AuthConfig struct {
    Method AuthMethod
    // Method-specific parameters
    Params map[string]any
}

type AuthChallenge struct {
    Type string // "redirect", "prompt", "apiToken"
    URL  string
    TTL  time.Duration
}

type AuthResponse struct {
    Type string
    Code string         // OAuth code, or method-specific token
    Params map[string]any
}
```

## Connection Interface (Go)

```go
// ConnectionValidator tests whether a connection is usable
type ConnectionValidator interface {
    // ValidateConnection checks connectivity and returns connection metadata
    ValidateConnection(ctx context.Context, conn *Connection) (*ConnectionStatus, error)
}

type ConnectionStatus struct {
    Healthy        bool
    LastChecked    time.Time
    Latency        time.Duration
    Error          *Error
    Metadata       map[string]any
}

type Connection struct {
    ID          string
    Integration string
    Config      map[string]any
    Credential  *CredentialRef
    Status      ConnectionStatus
}
```

## Operation Interface (Go)

```go
// OperationExecutor is implemented by integrations to execute operations
type OperationExecutor interface {
    // Capabilities returns the list of capabilities this integration provides
    Capabilities() []Capability

    // Execute runs an operation with validated input
    Execute(ctx context.Context, op *OperationRequest) (*OperationResult, error)
}

type OperationRequest struct {
    ID            string
    ConnectionID  string
    Integration   string
    OperationID   string // e.g. "repository.read_file"
    Input         map[string]any
    Caller        *IdentityContext
    IdempotencyKey string
    Timeout       time.Duration
    Metadata      map[string]any
}

type OperationResult struct {
    Data        map[string]any
    Error       *Error
    Metadata    map[string]any
    AuditMeta   AuditMetadata
}

type IdentityContext struct {
    UserID       string
    AppID        string
    DeviceID     string
    Permissions  []string
}
```

## Package Lifecycle Interface (Go)

```go
// Package defines the full lifecycle interface an integration implements
type Package interface {
    Authenticator
    ConnectionValidator
    OperationExecutor

    // Manifest returns the package manifest
    Manifest() *Manifest

    // Initialize prepares the integration (called once)
    Initialize(ctx context.Context, kernel RuntimeContext) error

    // Shutdown cleans up resources
    Shutdown(ctx context.Context) error
}

// RuntimeContext provides the integration access to kernel services
type RuntimeContext interface {
    // Logging
    Logger() Logger
    // Credential store access (never returns raw secrets to integrations directly)
    GetCredential(ctx context.Context, ref CredentialRef) (*CredentialMaterial, error)
    // Audit event emission
    EmitAuditEvent(ctx context.Context, event AuditEvent) error
    // Progress channel for long-running operations
    ProgressChan() chan OperationProgress
}
```

## Error Model

All integrations must convert provider-specific errors into normalized error
categories. The kernel uses these categories for consistent client-facing
responses and retry policy decisions.

```go
type ErrorCategory string

const (
    ErrorCategoryAuthFailed     ErrorCategory = "authentication_failed"
    ErrorCategoryAuthExpired    ErrorCategory = "authentication_expired"
    ErrorCategoryPermission     ErrorCategory = "permission_denied"
    ErrorCategoryNotFound       ErrorCategory = "not_found"
    ErrorCategoryRateLimited    ErrorCategory = "rate_limited"
    ErrorCategoryTimeout        ErrorCategory = "timeout"
    ErrorCategoryNetwork        ErrorCategory = "network_error"
    ErrorCategoryInvalidInput   ErrorCategory = "invalid_input"
    ErrorCategoryProvider       ErrorCategory = "provider_error"
    ErrorCategoryInternal       ErrorCategory = "internal_error"
)

type Error struct {
    Code    string         // Machine-readable error code
    Message string         // Human-readable message (safe for clients)
    Category ErrorCategory // Normalized category
    Details  map[string]any // Non-sensitive details
    Retryable bool          // Whether retrying is safe
}
```

## Operation States

```go
type OperationState string

const (
    StatePending        OperationState = "pending"
    StateValidating     OperationState = "validating"
    StateAwaitingAppr   OperationState = "awaiting_approval"
    StateQueued         OperationState = "queued"
    StateRunning        OperationState = "running"
    StateSucceeded      OperationState = "succeeded"
    StateFailed         OperationState = "failed"
    StateCancelRequested OperationState = "cancel_requested"
    StateCancelled      OperationState = "cancelled"
    StateTimedOut       OperationState = "timed_out"
    StateUnknownOutcome  OperationState = "unknown_outcome"
)
```

## TypeScript Contract

The SDK Package exposes the same contract in TypeScript:

```typescript
export interface Manifest {
  apiVersion: string
  metadata: {
    name: string
    displayName: string
    version: string
    packageApiVersion: string
    description?: string
  }
  spec: {
    supportedRuntimeVersions: string[]
    executionTarget: 'cloud' | 'local' | 'any'
    networkRequirements?: { outbound: string[]; inbound: string[] }
    dependencies?: string[]
    authMethods: string[]
    configSchema: object
    capabilities: Capability[]
    operations: OperationDef[]
  }
}

export interface Capability {
  name: string
  permissions: string[]
  description?: string
}

export interface OperationDef {
  id: string
  capability: string
  handler: string
  timeoutSeconds: number
  retryPolicy: 'safe' | 'unsafe' | 'none'
  idempotent: boolean
}

export interface Integration {
  manifest(): Manifest
  initialize(context: RuntimeContext): Promise<void>
  shutdown(): Promise<void>
  supportedAuthMethods(): AuthMethod[]
  startAuth(config: AuthConfig): Promise<AuthChallenge>
  completeAuth(response: AuthResponse): Promise<Credential>
  validateCredential(cred: Credential): Promise<void>
  refreshCredential(cred: Credential): Promise<Credential>
  revokeCredential(cred: Credential): Promise<void>
  validateConnection(conn: Connection): Promise<ConnectionStatus>
  capabilities(): Capability[]
  execute(request: OperationRequest): Promise<OperationResult>
}
```

## Package Isolation Policy

- **Trusted packages** (first-party integrations shipped with OmniConnector)
  may execute in-process within the Kernel Runtime.
- **Untrusted packages** must eventually run in isolated workers. The contract
  must not assume in-process execution for untrusted code.
- The `executionTarget` field in the manifest indicates where an operation
  must run: `cloud` (kernel backend), `local` (Local Agent), or `any`.
