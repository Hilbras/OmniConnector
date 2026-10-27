// Wire types for the OmniConnector Kernel API v1. These mirror the JSON the
// Go kernel emits; field names and optionality must stay in sync with
// packages/kernel-runtime and packages/shared-contracts.

/** Normalized error categories shared by every integration */
export type ErrorCategory =
  | "authentication_failed"
  | "authentication_expired"
  | "permission_denied"
  | "not_found"
  | "rate_limited"
  | "timeout"
  | "network_error"
  | "invalid_input"
  | "provider_error"
  | "internal_error"
  | "not_supported"
  | "conflict";

/** A normalized error, as returned in operation records and error responses */
export interface NormalizedError {
  code: string;
  message: string;
  category: ErrorCategory;
  details?: Record<string, unknown>;
  retryable: boolean;
}

/** Operation lifecycle states */
export type OperationState =
  | "pending"
  | "validating"
  | "awaiting_approval"
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "cancel_requested"
  | "cancelled"
  | "timed_out"
  | "unknown_outcome";

/** States after which an operation never changes again */
export const TERMINAL_STATES: ReadonlySet<OperationState> = new Set<OperationState>([
  "succeeded",
  "failed",
  "cancelled",
  "timed_out",
  "unknown_outcome",
]);

/** Reports whether an operation has reached a final state */
export function isTerminal(state: OperationState): boolean {
  return TERMINAL_STATES.has(state);
}

/** The kernel's record of one execution request. It never contains the operation input. */
export interface Operation {
  id: string;
  requestId?: string;
  integration: string;
  operationId: string;
  capability: string;
  connectionId?: string;
  callerUserId?: string;
  callerAppId?: string;
  executionTarget: string;
  authorization: string;
  idempotencyKey?: string;
  idempotent: boolean;
  timeoutSeconds: number;
  state: OperationState;
  result?: Record<string, unknown>;
  resultMetadata?: Record<string, unknown>;
  error?: NormalizedError;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface Capability {
  name: string;
  permissions: string[];
  description?: string;
}

export interface OperationDefinition {
  id: string;
  capability: string;
  handler: string;
  timeoutSeconds: number;
  retryPolicy: "safe" | "unsafe" | "none";
  idempotent: boolean;
  inputSchema?: Record<string, unknown>;
}

export interface IntegrationSummary {
  name: string;
  displayName: string;
  version: string;
  description?: string;
  capabilities: Capability[];
}

export interface IntegrationDetail extends IntegrationSummary {
  authMethods: string[];
  operations: OperationDefinition[];
}

export interface AuditEvent {
  id: string;
  actor: string;
  actorType: string;
  action: string;
  targetType: string;
  targetId: string;
  outcome: string;
  metadata?: Record<string, unknown>;
  timestamp: string;
}

export interface Health {
  status: string;
  version: string;
  runtimeVersion: string;
  uptimeSeconds: number;
  packages: number;
  runningOperations: number;
}

/** Body of POST /v1/operations */
export interface CreateOperationRequest {
  integration: string;
  operationId: string;
  connectionId?: string;
  input?: Record<string, unknown>;
  /** Lower the operation's timeout; it can never exceed the integration's declared limit */
  timeoutSeconds?: number;
}
