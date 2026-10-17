import {
  addOwnMessage,
  audioFormat,
  commentsToFetch,
  createMessageStream,
  decodeAudio,
  descriptionsToFetch,
  encodeAudio,
  encodeVoicePacket,
  handshake,
  initialState,
  ping,
  reduce,
  requestComments,
  requestDescriptions,
  requestTextures,
  sendText,
  setSelfComment,
  setSelfTexture,
  type Bytes,
  type MumbleMessage,
  type OutgoingAudio,
  type ServerState,
  type TextTarget,
} from "@kallisti/protocol";
import {
  deleteMessageRequest,
  editMessageRequest,
  historyRequest,
  httpBase,
  markReadRequest,
  typingRequest,
  searchRequest,
  pinRequest,
  directRequest,
  pinsRequest,
  startSearch,
  initialTextState,
  manageRequest,
  parseServerEvent,
  reduceText,
  sendRequest,
  uploadTokenRequest,
  voiceChatRequest,
  type ManageAction,
  type ServerEvent,
  type TextState,
} from "./textChannels";
import { untrack } from "svelte";
import { texturesToFetch } from "./avatar";
import type { SearchQuery } from "./search";
import { reactRequest } from "./reactions";
import { previewsRequest } from "./linkPreview";
import { createCapture } from "./voice/capture.svelte";
import { createStreamClient, streamingSupported } from "./stream/client.svelte";
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
  // It was in at some point (see shouldReconnect).
  let wasSynced = $state(false);
  // The close frame's reason (the desktop bridge explains refusals there).
  let closeReason = $state("");
  // Text channels from the Kallisti server (unavailable on plain Mumble).
  let text = $state.raw<TextState>(initialTextState);
  // One upload token request at a time (the UI uploads one file at a time).
  // Live text events for the UI (notifications), after the state took them.
  const textListeners = new Set<(ev: ServerEvent) => void>();
  let tokenWaiter: { resolve: (token: string) => void; reject: (err: Error) => void } | null = null;
  // Screen sharing (built-in server): its own connection, with a token
  // asked for here.
  // Answered in order; a waiter that timed out leaves the queue, so a late
  // answer cannot go to the wrong one.
  const streamTokenWaiters: ((token: string) => void)[] = [];
  const streamToken = () =>
    new Promise<string>((resolve, reject) => {
      if (ws.readyState !== WebSocket.OPEN) return reject(new Error("not connected"));
      const waiter = (t: string) => {
        clearTimeout(timeout);
        resolve(t);
      };
      const timeout = setTimeout(() => {
        streamTokenWaiters.splice(streamTokenWaiters.indexOf(waiter), 1);
        reject(new Error("no stream token from the server"));
      }, 10_000);
      streamTokenWaiters.push(waiter);
      ws.send(JSON.stringify({ type: "stream-token" }));
    });
  const base = httpBase(url);
  const streams = streamingSupported() ? createStreamClient(`${base.replace(/^http/, "ws")}/api/stream`, streamToken, audio) : null;
  const push = createMessageStream();
  const ws = new WebSocket(url);
  ws.binaryType = "arraybuffer";
  let timer: ReturnType<typeof setInterval> | undefined;

  // Long channel descriptions arrive as a hash: ask for each text once.
  let askedDescriptions: ReadonlySet<string> = new Set();
  const fetchDescriptions = () => {
    if (state.status !== "synced") return;
    const next = descriptionsToFetch(state, askedDescriptions);
    askedDescriptions = next.asked;
    if (next.ids.length) send(requestDescriptions(next.ids));
  };
  // Avatars too (see texturesToFetch).
  let askedTextures: ReadonlySet<string> = new Set();
  const fetchTextures = () => {
    if (state.status !== "synced") return;
    const next = texturesToFetch(state, askedTextures);
    askedTextures = next.asked;
    if (next.sessions.length) send(requestTextures(next.sessions));
  };
  // Comments only for users the UI shows one of (see commentsToFetch).
  let askedComments: ReadonlySet<string> = new Set();
  // Untracked: an effect that calls it should not rerun on each state change.
  const fetchComment = (session: number) =>
    untrack(() => {
      if (state.status !== "synced") return;
      const next = commentsToFetch(state, [session], askedComments);
      askedComments = next.asked;
      if (next.sessions.length) send(requestComments(next.sessions));
    });

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

  // Browsers do not freeze or discard tabs that hold a Web Lock: hold one
  // while connected (a name of our own, so other tabs do not wait).
  let releaseLock: (() => void) | undefined;
  void navigator.locks
    ?.request(`kallisti-connection-${crypto.randomUUID()}`, () => new Promise<void>((resolve) => (releaseLock = resolve)))
    .catch(() => {});
  ws.onopen = () => {
    ws.send(handshake(credentials));
    timer = setInterval(() => ws.send(ping(Date.now())), PING_INTERVAL_MS);
  };
  ws.onmessage = (event) => {
    // Text frames: the Kallisti server's JSON API. Binary: the Mumble stream.
    if (typeof event.data === "string") {
      const ev = parseServerEvent(event.data);
      if (ev?.type === "upload-token") tokenWaiter?.resolve(ev.token);
      else if (ev?.type === "stream-token") streamTokenWaiters.shift()?.(ev.token);
      else if (ev) text = reduceText(text, ev);
      if (ev) textListeners.forEach((fn) => fn(ev));
      return;
    }
    try {
      const messages = push(new Uint8Array(event.data as ArrayBuffer));
      messages.forEach(routeAudio);
      state = messages.reduce((s, m) => reduce(s, m, Date.now()), state);
      if (state.status === "synced") wasSynced = true;
      fetchDescriptions();
      fetchTextures();
    } catch (err) {
      // A broken frame leaves the stream out of sync: give up on it.
      console.error("protocol error:", err);
      ws.close();
    }
  };
  ws.onclose = (event) => {
    releaseLock?.();
    closeReason = event.reason;
    clearInterval(timer);
    tokenWaiter?.reject(new Error("connection closed"));
    capture?.close();
    streams?.close();
    playback?.close(); // the audio context stays: the app may reconnect with it
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
    get wasSynced() {
      return wasSynced;
    },
    account: credentials.account === true,
    get text() {
      return text;
    },
    // Asks which channel holds a voice channel's chat (built-in server).
    requestVoiceChat: (voice: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(voiceChatRequest(voice));
    },
    // Text channels and roles (admins and moderators).
    manage: (action: ManageAction) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(manageRequest(action));
    },
    requestHistory: (channel: number, before?: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(historyRequest(channel, before));
    },
    search: (query: SearchQuery) => {
      if (ws.readyState !== WebSocket.OPEN) return;
      text = startSearch(text); // the old results go
      ws.send(searchRequest(query));
    },
    previews: (messages: number[]) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(previewsRequest(messages));
    },
    react: (message: number, emoji: string, on: boolean) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(reactRequest(message, emoji, on));
    },
    typing: (channel: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(typingRequest(channel));
    },
    markRead: (channel: number, message: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(markReadRequest(channel, message));
    },
    editMessage: (message: number, text: string) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(editMessageRequest(message, text));
    },
    pin: (message: number, on: boolean) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(pinRequest(message, on));
    },
    requestDirect: (user: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(directRequest(user));
    },
    requestPins: (channel: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(pinsRequest(channel));
    },
    deleteMessage: (message: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(deleteMessageRequest(message));
    },
    sendToTextChannel: (channel: number, message: string, upload?: string, reply?: number, noPing?: boolean) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(sendRequest(channel, message, upload, reply, noPing));
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
    streams,
    send,
    sendText: (target: TextTarget, text: string) => {
      send(sendText(target, text));
      state = addOwnMessage(state, target, text, Date.now());
    },
    // Calls fn for each text event from the server; returns the way out.
    onTextEvent: (fn: (ev: ServerEvent) => void) => {
      textListeners.add(fn);
      return () => textListeners.delete(fn);
    },
    // Our avatar (empty: remove it).
    setTexture: (texture: Uint8Array) => {
      if (state.session !== null) send(setSelfTexture(state.session, texture));
    },
    // Our comment (the bio), from plain text; a user's long comment.
    setComment: (text: string) => {
      if (state.session !== null) send(setSelfComment(state.session, text));
    },
    fetchComment,
    close: () => ws.close(),
  };
};
