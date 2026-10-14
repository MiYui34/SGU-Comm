import {
  addOwnMessage,
  audioFormat,
  createMessageStream,
  decodeAudio,
  encodeAudio,
  encodeVoicePacket,
  handshake,
  initialState,
  missingDescriptions,
  ping,
  reduce,
  requestDescriptions,
  sendText,
  type Bytes,
  type MumbleMessage,
  type OutgoingAudio,
  type ServerState,
  type TextTarget,
} from "@kallisti/protocol";
import {
  historyRequest,
  httpBase,
  initialTextState,
  parseServerEvent,
  reduceText,
  sendRequest,
  uploadTokenRequest,
  type TextState,
} from "./textChannels";
import { createCapture } from "./voice/capture.svelte";
import { createPlayback } from "./voice/playback.svelte";

// The server drops clients after 30 s without a ping.
const PING_INTERVAL_MS = 15_000;

// account: logged in to a Kallisti account (the server connects with the
// account's certificate, and the client registers it on Murmur).
// server: the desktop app's server address ("host" or "host:port").
export type Credentials = { username: string; password?: string; account?: boolean; server?: string };

export type Connection = ReturnType<typeof connect>;

// Thin effectful shell around the pure protocol code. Owns the audio
// context (null: no voice support), so voice lives and dies with the socket.
export const connect = (url: string, credentials: Credentials, audio: AudioContext | null) => {
  let state = $state.raw<ServerState>(initialState);
  let closed = $state(false);
  // The close frame's reason (the desktop bridge explains refusals there).
  let closeReason = $state("");
  // Text channels from the Kallisti server (unavailable on plain Mumble).
  let text = $state.raw<TextState>(initialTextState);
  // One upload token request at a time (the UI uploads one file at a time).
  let tokenWaiter: { resolve: (token: string) => void; reject: (err: Error) => void } | null = null;
  const base = httpBase(url);
  const push = createMessageStream();
  const ws = new WebSocket(url);
  ws.binaryType = "arraybuffer";
  let timer: ReturnType<typeof setInterval> | undefined;

  // Long channel descriptions arrive as a hash: ask for each text once.
  const askedDescriptions = new Set<number>();
  const fetchDescriptions = () => {
    if (state.status !== "synced") return;
    const ids = missingDescriptions(state).filter((id) => !askedDescriptions.has(id));
    if (!ids.length) return;
    ids.forEach((id) => askedDescriptions.add(id));
    send(requestDescriptions(ids));
  };

  const send = (bytes: Bytes) => {
    if (ws.readyState === WebSocket.OPEN) ws.send(bytes);
  };
  const sendAudio = (packet: OutgoingAudio) => {
    if (state.status === "synced") send(encodeVoicePacket(encodeAudio(audioFormat(state), packet)));
  };

  const playback = audio && createPlayback(audio);
  // The UI starts capture, with the device from the settings.
  const capture = audio && createCapture(audio, sendAudio);

  // Voice and leaving users go to the playback side. State stays pure.
  const routeAudio = (msg: MumbleMessage) => {
    if (!playback) return;
    if (msg.type === "UDPTunnel") {
      const packet = decodeAudio(audioFormat(state), msg.packet);
      if (packet) playback.handle(packet);
    } else if (msg.type === "UserRemove") {
      playback.forget(msg.message.session);
    }
  };

  ws.onopen = () => {
    ws.send(handshake(credentials));
    timer = setInterval(() => ws.send(ping(Date.now())), PING_INTERVAL_MS);
  };
  ws.onmessage = (event) => {
    // Text frames: the Kallisti server's JSON API. Binary: the Mumble stream.
    if (typeof event.data === "string") {
      const ev = parseServerEvent(event.data);
      if (ev?.type === "upload-token") tokenWaiter?.resolve(ev.token);
      else if (ev) text = reduceText(text, ev);
      return;
    }
    try {
      const messages = push(new Uint8Array(event.data as ArrayBuffer));
      messages.forEach(routeAudio);
      state = messages.reduce((s, m) => reduce(s, m, Date.now()), state);
      fetchDescriptions();
    } catch (err) {
      // A broken frame leaves the stream out of sync: give up on it.
      console.error("protocol error:", err);
      ws.close();
    }
  };
  ws.onclose = (event) => {
    closeReason = event.reason;
    clearInterval(timer);
    tokenWaiter?.reject(new Error("connection closed"));
    capture?.close();
    playback?.close();
    void audio?.close();
    closed = true;
  };

  return {
    get state() {
      return state;
    },
    get closed() {
      return closed;
    },
    get closeReason() {
      return closeReason;
    },
    account: credentials.account === true,
    get text() {
      return text;
    },
    requestHistory: (channel: number, before?: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(historyRequest(channel, before));
    },
    sendToTextChannel: (channel: number, message: string, upload?: string) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(sendRequest(channel, message, upload));
    },
    // The server's HTTP origin (uploads and files).
    httpBase: base,
    // Uploads a file and returns its key, to attach with sendToTextChannel.
    uploadFile: async (file: File): Promise<string> => {
      if (ws.readyState !== WebSocket.OPEN) throw new Error("not connected");
      if (tokenWaiter) throw new Error("an upload is already starting");
      const token = await new Promise<string>((resolve, reject) => {
        // The server may answer with an error instead (rate limit): give up
        // after a while, so the next upload can try again.
        const timeout = setTimeout(() => done(() => reject(new Error("no upload token from the server"))), 10_000);
        const done = (settle: () => void) => {
          clearTimeout(timeout);
          tokenWaiter = null;
          settle();
        };
        tokenWaiter = {
          resolve: (t) => done(() => resolve(t)),
          reject: (err) => done(() => reject(err)),
        };
        ws.send(uploadTokenRequest());
      });
      const res = await fetch(`${base}/api/upload`, {
        method: "POST",
        headers: {
          "Content-Type": file.type || "application/octet-stream",
          "X-Filename": encodeURIComponent(file.name),
          "X-Upload-Token": token,
        },
        body: file,
      });
      const body = (await res.json().catch(() => ({}))) as { key?: string; error?: string };
      if (!res.ok || !body.key) throw new Error(body.error ?? `upload failed (${res.status})`);
      return body.key;
    },
    playback,
    capture,
    send,
    sendText: (target: TextTarget, text: string) => {
      send(sendText(target, text));
      state = addOwnMessage(state, target, text, Date.now());
    },
    close: () => ws.close(),
  };
};
