// Minimal-runtime check: install the packed tarball and run this with the
// oldest supported Node (see "engines") to prove the package loads and works.
import { OmniConnectorClient, OmniConnectorError, VERSION } from "@hilbras/omniconnector";
const c = new OmniConnectorClient({ baseUrl: "http://127.0.0.1:1", token: "t", timeoutMs: 500 });
try { await c.integrations.list(); process.exit(1); } catch (e) {
  if (!(e instanceof OmniConnectorError) || e.category !== "network_error") { console.error(e); process.exit(1); }
}
console.log(`ok ${VERSION} on node ${process.version}`);
