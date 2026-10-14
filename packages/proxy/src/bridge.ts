import { createServer } from "node:http";
import type { AddressInfo, Socket } from "node:net";
import { WebSocketServer, type WebSocket } from "ws";
import { isOriginAllowed } from "./config.ts";

export type BridgeOptions = {
  port: number;
  allowedOrigins: readonly string[];
  maxConnections: number;
  connectUpstream: () => Socket;
};

export type Bridge = { port: number; close: () => Promise<void> };

const CLOSE_UNSUPPORTED = 1003;
const CLOSE_UPSTREAM_ERROR = 1011;
// Same limit as Mumble frames.
const MAX_PAYLOAD = 8 * 1024 * 1024;
// Keep queues short: for voice, late data is worse than a stalled sender.
const HIGH_WATER = 256 * 1024;

// Moves raw bytes between one WebSocket and one upstream socket. Mumble
// framing is done by the browser, so chunk boundaries do not matter here.
const pipe = (ws: WebSocket, upstream: Socket) => {
  const resumeUpstream = () => {
    if (upstream.isPaused() && ws.bufferedAmount < HIGH_WATER) upstream.resume();
  };
  upstream.on("data", (chunk) => {
    ws.send(chunk, resumeUpstream);
    if (ws.bufferedAmount >= HIGH_WATER) upstream.pause();
  });
  upstream.on("drain", () => ws.resume());
  upstream.on("error", () => ws.close(CLOSE_UPSTREAM_ERROR));
  upstream.on("close", () => ws.close());
  ws.on("message", (data, isBinary) => {
    if (!isBinary) return ws.close(CLOSE_UNSUPPORTED);
    if (!upstream.write(data as Buffer)) ws.pause();
  });
  // Without this listener, one malformed frame crashes the process.
  ws.on("error", () => upstream.destroy());
  ws.on("close", () => upstream.destroy());
};

export const startBridge = async (opts: BridgeOptions): Promise<Bridge> => {
  const http = createServer();
  const wss: WebSocketServer = new WebSocketServer({
    server: http,
    maxPayload: MAX_PAYLOAD,
    // Compression stays off (the ws default): it adds latency to voice.
    perMessageDeflate: false,
    verifyClient: ({ origin }: { origin?: string }, done: (ok: boolean, code?: number) => void) => {
      if (!isOriginAllowed(opts.allowedOrigins, origin)) return done(false, 403);
      if (wss.clients.size >= opts.maxConnections) return done(false, 503);
      done(true);
    },
  });
  wss.on("connection", (ws) => {
    try {
      pipe(ws, opts.connectUpstream());
    } catch (err) {
      console.error("upstream connect failed:", err);
      ws.close(CLOSE_UPSTREAM_ERROR);
    }
  });
  await new Promise<void>((resolve) => http.listen(opts.port, resolve));
  return {
    port: (http.address() as AddressInfo).port,
    close: () =>
      new Promise((resolve) => {
        wss.clients.forEach((ws) => ws.terminate());
        wss.close(() => http.close(() => resolve()));
      }),
  };
};
