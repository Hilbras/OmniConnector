import type { ErrorCategory, NormalizedError } from "./types.js";

/**
 * Thrown when a request is rejected by the API or never reaches it.
 *
 * An operation that was accepted but then failed is NOT thrown: it is
 * returned as an Operation whose `state` is "failed" (or "timed_out",
 * "unknown_outcome", …) with an `error` field.
 */
export class OmniConnectorError extends Error {
  /** HTTP status, or 0 when no response was received */
  readonly status: number;
  readonly code: string;
  readonly category: ErrorCategory;
  /** Whether repeating the same request may succeed */
  readonly retryable: boolean;
  /** Correlation ID to quote when reporting the failure */
  readonly requestId: string | undefined;
  readonly details: Record<string, unknown> | undefined;

  constructor(status: number, error: NormalizedError, requestId?: string, options?: { cause?: unknown }) {
    super(error.message, options);
    this.name = "OmniConnectorError";
    this.status = status;
    this.code = error.code;
    this.category = error.category;
    this.retryable = error.retryable;
    this.requestId = requestId;
    this.details = error.details;
  }
}
