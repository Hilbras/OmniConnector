import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";
import { OmniConnectorClient, OmniConnectorError, VERSION, isTerminal } from "../src/index.js";

function mockFetch(status: number, body: unknown, headers: Record<string, string> = {}) {
  return vi.fn(async (_url: string | URL | Request, _init?: RequestInit) =>
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json", ...headers },
    }),
  );
}

function client(fetchImpl: typeof fetch, extra: Partial<ConstructorParameters<typeof OmniConnectorClient>[0]> = {}) {
  return new OmniConnectorClient({ baseUrl: "https://api.example.com/", token: "tok", fetch: fetchImpl, ...extra });
}

describe("OmniConnectorClient", () => {
  it("validates options", () => {
    expect(() => new OmniConnectorClient({ baseUrl: "not a url", token: "t" })).toThrow(TypeError);
    expect(() => new OmniConnectorClient({ baseUrl: "ftp://x", token: "t" })).toThrow(/http or https/);
    expect(() => new OmniConnectorClient({ baseUrl: "https://x", token: "" })).toThrow(/token/);
  });

  it("VERSION matches package.json", () => {
    const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8")) as { version: string };
    expect(VERSION).toBe(pkg.version);
  });

  it("creates operations with auth, idempotency and request id headers", async () => {
    const f = mockFetch(201, { id: "op_1", state: "succeeded" });
    const op = await client(f as unknown as typeof fetch).operations.create(
      { integration: "test-integration", operationId: "echo", input: { message: "hi" } },
      { idempotencyKey: "key-1", requestId: "req-1" },
    );
    expect(op.id).toBe("op_1");
    const [url, init] = f.mock.calls[0]!;
    expect(url).toBe("https://api.example.com/v1/operations");
    const headers = init!.headers as Record<string, string>;
    expect(init!.method).toBe("POST");
    expect(headers.Authorization).toBe("Bearer tok");
    expect(headers["Idempotency-Key"]).toBe("key-1");
    expect(headers["X-Request-ID"]).toBe("req-1");
    expect(headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(init!.body as string)).toEqual({ integration: "test-integration", operationId: "echo", input: { message: "hi" } });
  });

  it("does not send credentials to unauthenticated endpoints", async () => {
    const f = mockFetch(200, { status: "healthy" });
    await client(f as unknown as typeof fetch).health();
    expect((f.mock.calls[0]![1]!.headers as Record<string, string>).Authorization).toBeUndefined();
  });

  it("escapes path parameters", async () => {
    const f = mockFetch(200, { id: "x" });
    await client(f as unknown as typeof fetch).operations.get("../admin?x=1");
    expect(f.mock.calls[0]![0]).toBe("https://api.example.com/v1/operations/..%2Fadmin%3Fx%3D1");
  });

  it("throws normalized API errors with the request id", async () => {
    const f = mockFetch(
      403,
      { error: { code: "permission_denied", message: "missing required permission: test.write", category: "permission_denied", retryable: false, requestId: "req-9" } },
      { "X-Request-ID": "req-9" },
    );
    const err = await client(f as unknown as typeof fetch)
      .operations.create({ integration: "t", operationId: "write" })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(OmniConnectorError);
    expect(err).toMatchObject({ status: 403, code: "permission_denied", category: "permission_denied", retryable: false, requestId: "req-9" });
  });

  it("returns failed operations instead of throwing", async () => {
    const f = mockFetch(201, { id: "op_2", state: "failed", error: { code: "x", message: "m", category: "rate_limited", retryable: true } });
    const op = await client(f as unknown as typeof fetch).operations.create({ integration: "t", operationId: "fail" });
    expect(op.state).toBe("failed");
    expect(op.error?.retryable).toBe(true);
  });

  it("handles non-JSON error bodies", async () => {
    const f = vi.fn(async () => new Response("<html>bad gateway</html>", { status: 502 }));
    await expect(client(f as unknown as typeof fetch).integrations.list()).rejects.toMatchObject({
      status: 502,
      code: "http_error",
      category: "internal_error",
      retryable: true,
    });
  });

  it("maps network failures and timeouts", async () => {
    const down = vi.fn(async () => {
      throw new TypeError("fetch failed");
    });
    await expect(client(down as unknown as typeof fetch).integrations.list()).rejects.toMatchObject({
      status: 0,
      category: "network_error",
    });

    const hang = vi.fn((_u: unknown, init?: RequestInit) =>
      new Promise<Response>((_, reject) => init!.signal!.addEventListener("abort", () => reject(init!.signal!.reason))),
    );
    await expect(client(hang as unknown as typeof fetch, { timeoutMs: 20 }).integrations.list()).rejects.toMatchObject({
      category: "timeout",
      code: "client_timeout",
    });
  });

  it("propagates caller aborts unchanged", async () => {
    const hang = vi.fn((_u: unknown, init?: RequestInit) =>
      new Promise<Response>((_, reject) => init!.signal!.addEventListener("abort", () => reject(init!.signal!.reason))),
    );
    const ac = new AbortController();
    const p = client(hang as unknown as typeof fetch).integrations.list({ signal: ac.signal });
    ac.abort(new Error("stop"));
    await expect(p).rejects.toThrow("stop");
  });

  it("waits until an operation is terminal", async () => {
    const states = ["queued", "running", "succeeded"];
    const f = vi.fn(async () => new Response(JSON.stringify({ id: "op_3", state: states.shift() }), { status: 200 }));
    const op = await client(f as unknown as typeof fetch).operations.wait("op_3", { intervalMs: 1 });
    expect(op.state).toBe("succeeded");
    expect(f).toHaveBeenCalledTimes(3);

    const stuck = vi.fn(async () => new Response(JSON.stringify({ id: "op_4", state: "running" }), { status: 200 }));
    await expect(client(stuck as unknown as typeof fetch).operations.wait("op_4", { intervalMs: 5, timeoutMs: 12 })).rejects.toMatchObject({
      code: "wait_timeout",
    });
  });

  it("classifies terminal states", () => {
    expect(isTerminal("unknown_outcome")).toBe(true);
    expect(isTerminal("cancel_requested")).toBe(false);
  });
});
