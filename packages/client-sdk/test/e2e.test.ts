// End-to-end tests against a real Kernel API binary. They run when
// OMNICONNECTOR_API_BIN points at a built kernel-api (`make build`).
import { type ChildProcess, spawn } from "node:child_process";
import { createServer } from "node:net";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { OmniConnectorClient, OmniConnectorError } from "../src/index.js";

const bin = process.env.OMNICONNECTOR_API_BIN;
const token = "e2e-token-0123456789abcdef0123456789";

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = createServer();
    srv.listen(0, "127.0.0.1", () => {
      const addr = srv.address();
      srv.close(() => (typeof addr === "object" && addr ? resolve(addr.port) : reject(new Error("no port"))));
    });
  });
}

describe.skipIf(!bin)("against a live kernel-api", () => {
  let proc: ChildProcess;
  let api: OmniConnectorClient;

  beforeAll(async () => {
    const port = await freePort();
    proc = spawn(bin!, [], {
      env: {
        ...process.env,
        KERNEL_HTTP_ADDR: `127.0.0.1:${port}`,
        KERNEL_DEV_TOKEN: token,
        KERNEL_DEV_PERMISSIONS: "operation.execute,operation.cancel,test.read,test.write,audit.read",
        KERNEL_ENABLE_TEST_INTEGRATION: "true",
        KERNEL_DATABASE_DSN: "",
        KERNEL_LOG_LEVEL: "error",
      },
      stdio: "ignore",
    });
    api = new OmniConnectorClient({ baseUrl: `http://127.0.0.1:${port}`, token });
    for (let i = 0; i < 50; i++) {
      try {
        await api.ready();
        return;
      } catch {
        await new Promise((r) => setTimeout(r, 100));
      }
    }
    throw new Error("kernel-api did not become ready");
  });

  afterAll(() => {
    proc?.kill();
  });

  it("discovers integrations", async () => {
    const list = await api.integrations.list();
    expect(list.map((i) => i.name)).toContain("test-integration");
    const detail = await api.integrations.get("test-integration");
    expect(detail.operations.map((o) => o.id)).toContain("echo");
    expect((await api.integrations.capabilities("test-integration")).length).toBe(2);
  });

  it("runs an operation and reads it back", async () => {
    const op = await api.operations.create(
      { integration: "test-integration", operationId: "echo", input: { message: "hello" } },
      { requestId: "e2e-req-1" },
    );
    expect(op.state).toBe("succeeded");
    expect(op.result).toEqual({ message: "hello" });
    expect(op.requestId).toBe("e2e-req-1");
    expect((await api.operations.get(op.id)).state).toBe("succeeded");
  });

  it("returns normalized failures and rejections", async () => {
    const failed = await api.operations.create({ integration: "test-integration", operationId: "fail", input: { category: "not_found" } });
    expect(failed.state).toBe("failed");
    expect(failed.error?.category).toBe("not_found");

    const err = await api.operations
      .create({ integration: "test-integration", operationId: "echo", input: { message: 5 } })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(OmniConnectorError);
    expect((err as OmniConnectorError).category).toBe("invalid_input");
    expect((err as OmniConnectorError).requestId).toBeTruthy();
  });

  it("deduplicates with idempotency keys", async () => {
    const req = { integration: "test-integration", operationId: "write", input: {} };
    const a = await api.operations.create(req, { idempotencyKey: "e2e-key" });
    const b = await api.operations.create(req, { idempotencyKey: "e2e-key" });
    expect(b.id).toBe(a.id);
  });

  it("cancels a running operation", async () => {
    const pending = api.operations.create({ integration: "test-integration", operationId: "write", input: { milliseconds: 10_000 } });
    let id: string | undefined;
    for (let i = 0; i < 50 && !id; i++) {
      id = (await api.operations.list()).find((o) => o.state === "running")?.id;
      if (!id) await new Promise((r) => setTimeout(r, 20));
    }
    expect(id).toBeDefined();
    expect((await api.operations.cancel(id!)).state).toBe("cancel_requested");
    expect((await pending).state).toBe("unknown_outcome");
    expect((await api.operations.wait(id!, { intervalMs: 10 })).state).toBe("unknown_outcome");
  });

  it("lists audit events", async () => {
    const events = await api.auditEvents.list({ limit: 5 });
    expect(events.length).toBeGreaterThan(0);
    expect(events[0]!.actor).toBe("dev-user");
  });

  it("rejects a bad token", async () => {
    const bad = new OmniConnectorClient({ baseUrl: (api as unknown as { baseUrl: string }).baseUrl, token: "wrong" });
    await expect(bad.integrations.list()).rejects.toMatchObject({ status: 401, category: "authentication_failed" });
  });
});
