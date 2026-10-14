import { isIP } from "node:net";
import tls from "node:tls";
import { startBridge } from "./bridge.ts";
import { parseConfig } from "./config.ts";

const result = parseConfig(process.env);
if (!result.ok) {
  console.error(result.error);
  process.exit(1);
}
const { config } = result;

if (config.allowedOrigins.includes("*")) {
  console.warn("ALLOWED_ORIGINS is *: every origin can connect (development only)");
}

const bridge = await startBridge({
  port: config.listenPort,
  allowedOrigins: config.allowedOrigins,
  maxConnections: config.maxConnections,
  connectUpstream: () =>
    tls
      .connect({
        host: config.mumbleHost,
        port: config.mumblePort,
        // SNI does not allow IP addresses.
        servername: isIP(config.mumbleHost) ? undefined : config.mumbleHost,
        rejectUnauthorized: config.rejectUnauthorized,
      })
      .setNoDelay(true),
});

console.log(
  `proxy on :${bridge.port} -> ${config.mumbleHost}:${config.mumblePort}`,
);
