import { once } from "node:events";
import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import WebSocket from "ws";
import { startBridge, type Bridge } from "../src/bridge.ts";

const cleanup: (() => unknown)[] = [];
afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((f) => f()));
});

const echoServer = async () => {
  const sockets = new Set<net.Socket>();
  const server = net.createServer((s) => {
    sockets.add(s);
    s.on("close", () => sockets.delete(s));
    s.pipe(s);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  cleanup.push(() => {
    sockets.forEach((s) => s.destroy());
    return new Promise((r) => server.close(r));
  });
  return { port: (server.address() as net.AddressInfo).port, sockets };
};

const bridgeTo = async (
  upstreamPort: number,
  { allowedOrigins = ["*"], maxConnections = 10 } = {},
) => {
  const bridge: Bridge = await startBridge({
    port: 0,
    allowedOrigins,
    maxConnections,
    connectUpstream: () => net.connect(upstreamPort, "127.0.0.1"),
  });
  cleanup.push(() => bridge.close());
  return bridge;
};

const connect = async (port: number, origin?: string) => {
  const ws = new WebSocket(`ws://127.0.0.1:${port}`, { origin });
  cleanup.push(() => ws.terminate());
  return ws;
};

describe("bridge", () => {
  it("forwards binary data both ways", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port);
    const ws = await connect(bridge.port);
    await once(ws, "open");
    ws.send(new Uint8Array([1, 2, 3]));
    const [data] = await once(ws, "message");
    expect(new Uint8Array(data as Buffer)).toEqual(new Uint8Array([1, 2, 3]));
  });

  it("closes the WebSocket when the upstream closes", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port);
    const ws = await connect(bridge.port);
    await once(ws, "open");
    await expect.poll(() => echo.sockets.size).toBe(1);
    echo.sockets.forEach((s) => s.end());
    await once(ws, "close");
  });

  it("closes the upstream when the WebSocket closes", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port);
    const ws = await connect(bridge.port);
    await once(ws, "open");
    await expect.poll(() => echo.sockets.size).toBe(1);
    ws.close();
    await expect.poll(() => echo.sockets.size).toBe(0);
  });

  it("closes the WebSocket when the upstream cannot connect", async () => {
    const bridge = await bridgeTo(1);
    const ws = await connect(bridge.port);
    const [code] = await once(ws, "close");
    expect(code).toBe(1011);
  });

  it("rejects origins that are not allowed", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port, { allowedOrigins: ["https://good.example"] });
    const ws = await connect(bridge.port, "https://evil.example");
    const [err] = await once(ws, "error");
    expect(String(err)).toMatch(/403/);
    expect(echo.sockets.size).toBe(0);
  });

  it("closes the connection on text frames", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port);
    const ws = await connect(bridge.port);
    await once(ws, "open");
    const [code] = await (ws.send("hello"), once(ws, "close"));
    expect(code).toBe(1003);
  });

  it("survives a malformed WebSocket frame", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port);
    const raw = net.connect(bridge.port, "127.0.0.1");
    cleanup.push(() => raw.destroy());
    raw.write(
      "GET / HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
        "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n",
    );
    await once(raw, "data");
    raw.write(Buffer.from([0x82, 0x01, 0x00])); // unmasked client frame: protocol error
    await once(raw, "close");
    const ws = await connect(bridge.port);
    await once(ws, "open");
  });

  it("closes messages larger than the Mumble frame limit", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port);
    const ws = await connect(bridge.port);
    await once(ws, "open");
    ws.send(new Uint8Array(9 * 1024 * 1024));
    const [code] = await once(ws, "close");
    expect(code).toBe(1009);
  });

  it("rejects connections above the limit", async () => {
    const echo = await echoServer();
    const bridge = await bridgeTo(echo.port, { maxConnections: 1 });
    const first = await connect(bridge.port);
    await once(first, "open");
    const second = await connect(bridge.port);
    const [err] = await once(second, "error");
    expect(String(err)).toMatch(/503/);
  });
});
