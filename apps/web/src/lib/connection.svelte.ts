import { threadRequest, type ThreadAction } from "./threads";
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
  deleteEmojiRequest,
  addUserEmojiRequest,
  deleteUserEmojiRequest,
  removeEmojiImageRequest,
  renameUserEmojiRequest,
  userEmojiTokenRequest,
  deleteMessageRequest,
  editMessageRequest,
  emojiTokenRequest,
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
  resyncRequest,
  forgetStatus,
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
import { bioRequest } from "./members";
import { awayRequest, setStatusRequest, type CustomStatus, type Presence } from "./presence";
import { createCapture } from "./voice/capture.svelte";
import { createStreamClient, streamingSupported } from "./stream/client.svelte";
import { createPlayback } from "./voice/playback.svelte";
import { createConnectionInfo } from "./connectionInfo.svelte";
import { exposePlayback } from "./perfHook";
import { transportOf } from "./connectionQuality";
import { recordingUrl } from "./recording";
import { answer, ANSWER_MS, confirmsManage, type Via } from "./saveFeedback";

// The server drops clients after 30 s without a ping. Each ping also
// measures the round trip (see connectionInfo), so as in Mumble: often.
const PING_INTERVAL_MS = 5_000;
// Without ready by then, the server has no text channels (see fetchTextures).
const NO_READY_MS = 3_000;

// account: logged in to a Kallisti account (the server connects with the
// account's certificate, and the client registers it on Murmur).
// server: the desktop app's server address ("host" or "host:port").
export type Credentials = { username: string; password?: string; account?: boolean; server?: string };

export type Connection = ReturnType<typeof connect>;

// Thin effectful shell around the pure protocol code. Owns the audio
// context (null: no voice support), so voice lives and dies with the socket.
// protocols: WebSocket subprotocols (the desktop app's session token, see
// lib/desktop kallistiSocket).
export const connect = (url: string, credentials: Credentials, audio: AudioContext | null, protocols: string[] = []) => {
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
  // emoji: for a custom emoji (a refusal comes as a manage error).
  let tokenWaiter: { resolve: (token: string) => void; reject: (err: Error) => void; emoji: boolean } | null = null;
  // Tokens asked for over the WebSocket (type: the request and answer).
  // Answered in order; a waiter that timed out leaves the queue, so a late
  // answer cannot go to the wrong one.
  type TokenType = "stream-token" | "recording-token";
  const tokenWaiters: Record<TokenType, ((token: string) => void)[]> = { "stream-token": [], "recording-token": [] };
  const askToken = (type: TokenType) =>
    new Promise<string>((resolve, reject) => {
      if (ws.readyState !== WebSocket.OPEN) return reject(new Error("not connected"));
      const waiters = tokenWaiters[type];
      const waiter = (t: string) => {
        clearTimeout(timeout);
        resolve(t);
      };
      const timeout = setTimeout(() => {
        waiters.splice(waiters.indexOf(waiter), 1);
        reject(new Error(`no ${type} from the server`));
      }, 10_000);
      waiters.push(waiter);
      ws.send(JSON.stringify({ type }));
    });
  // Requests a control waits on (see lib/saveFeedback): settled by the
  // server's answer, its refusal, or no answer in time. A refusal goes to
  // the control, not to the notice.
  type Check = { via: "text"; confirms: (ev: ServerEvent) => boolean } | { via: "mumble"; holds: (s: ServerState) => boolean };
  type Waiter = Check & { done: (error?: string) => void };
  let awaiting: Waiter[] = [];
  const wait = (check: Check, request: () => void) =>
    new Promise<void>((resolve, reject) => {
      if (ws.readyState !== WebSocket.OPEN) return reject(new Error("Not connected"));
      const timeout = setTimeout(() => waiter.done("No answer from the server"), ANSWER_MS);
      const waiter: Waiter = {
        ...check,
        done: (error) => {
          clearTimeout(timeout);
          awaiting = awaiting.filter((w) => w !== waiter);
          if (error === undefined) resolve();
          else reject(new Error(error));
        },
      };
      awaiting = [...awaiting, waiter];
      request();
    });
  // Settles the waiters an answer is for; true if one took the refusal.
  const settleWaiters = (via: Via, refusal: string | null, matches: (w: Waiter) => boolean) => {
    if (!awaiting.some((w) => w.via === via)) return false;
    const r = answer(awaiting, via, refusal !== null, matches);
    awaiting = r.rest;
    r.confirmed.forEach((w) => w.done());
    r.refused?.done(refusal ?? undefined);
    return r.refused !== undefined;
  };
  // Screen sharing (built-in server): its own connection, with a token
  // asked for here.
  const streamToken = () => askToken("stream-token");
  const base = httpBase(url);
  const streams = streamingSupported() ? createStreamClient(`${base.replace(/^http/, "ws")}/api/stream`, streamToken, audio) : null;
  const push = createMessageStream();
  const ws = protocols.length ? new WebSocket(url, protocols) : new WebSocket(url);
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
  // Avatars too (see texturesToFetch), once ready tells whether they come
  // over HTTP, or a while after the sync without ready (no Kallisti server).
  let askedTextures: ReadonlySet<string> = new Set();
  let noReady = false;
  const fetchTextures = () => {
    if (state.status !== "synced" || !(text.available || noReady)) return;
    const next = texturesToFetch(state, askedTextures, text.avatars);
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
  const playbackStats = () => playback?.stats() ?? { good: 0, late: 0, lost: 0 };
  const info = createConnectionInfo(transportOf(url), playbackStats);
  exposePlayback(playbackStats);
  // Not in yet: only to stay connected. During the handshake the answer
  // waits for the login, and the ping would look slow.
  const sendPing = () => {
    if (!wasSynced) return ws.send(ping(0));
    const { timestamp, stats } = info.ping(performance.now());
    ws.send(ping(timestamp, stats));
  };

  // Voice and leaving users go to the playback side, pings to the
  // connection info. State stays pure.
  const routeAudio = (msg: MumbleMessage) => {
    if (msg.type === "Ping") info.pong(Number(msg.message.timestamp), performance.now());
    if (!playback) return;
    // Not before the sync (the server may send voice first). Playback
    // also waits for each speaker's local mute and volume (setUserGain).
    if (msg.type === "UDPTunnel" && state.status === "synced") {
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
    timer = setInterval(sendPing, PING_INTERVAL_MS);
  };
  ws.onmessage = (event) => {
    // Text frames: the Kallisti server's JSON API. Binary: the Mumble stream.
    if (typeof event.data === "string") {
      const ev = parseServerEvent(event.data);
      if (ev?.type === "upload-token") tokenWaiter?.resolve(ev.token);
      else if (ev?.type === "error" && ev.about === "manage" && tokenWaiter?.emoji) tokenWaiter.reject(new Error(ev.message));
      else if (ev?.type === "stream-token" || ev?.type === "recording-token") tokenWaiters[ev.type].shift()?.(ev.token);
      else if (ev && !settleWaiters("text", ev.type === "error" && ev.about === "manage" ? ev.message : null, (w) => w.via === "text" && w.confirms(ev)))
        text = reduceText(text, ev);
      if (ev?.type === "ready") fetchTextures();
      if (ev?.type === "resync") ws.send(resyncRequest); // lost events: the state comes again
      if (ev) textListeners.forEach((fn) => fn(ev));
      return;
    }
    try {
      const messages = push(new Uint8Array(event.data as ArrayBuffer));
      messages.forEach(routeAudio);
      for (const m of messages) if (m.type === "UserRemove") text = forgetStatus(text, m.message.session);
      const before = state;
      state = messages.reduce((s, m) => reduce(s, m, Date.now()), state);
      const denied = messages.some((m) => m.type === "PermissionDenied") ? (state.notice?.text ?? "") : null;
      if (settleWaiters("mumble", denied, (w) => w.via === "mumble" && w.holds(state))) state = { ...state, notice: before.notice };
      if (state.status === "synced" && !wasSynced) {
        wasSynced = true;
        // The first measured one, after the first render: a busy page
        // reads the answer late.
        setTimeout(sendPing, 1000);
        setTimeout(() => ((noReady = true), fetchTextures()), NO_READY_MS);
      }
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
    awaiting.forEach((w) => w.done("Not connected"));
    capture?.close();
    streams?.close();
    playback?.close(); // the audio context stays: the app may reconnect with it
    closed = true;
  };

  // An upload: a token over the WebSocket, then the file over HTTP.
  const upload = async (tokenRequest: string, path: string, file: File, emoji: boolean) => {
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
        emoji,
      };
      ws.send(tokenRequest);
    });
    const res = await fetch(`${base}${path}`, {
      method: "POST",
      headers: {
        "Content-Type": file.type || "application/octet-stream",
        "X-Filename": encodeURIComponent(file.name),
        "X-Upload-Token": token,
      },
      body: file,
    });
    const body = (await res.json().catch(() => ({}))) as { key?: string; error?: string };
    if (!res.ok) throw new Error(body.error ?? `upload failed (${res.status})`);
    return body;
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
    // Ping, late audio and transport (see connectionInfo).
    get info() {
      return info.info;
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
    // The same, for a control that waits on the answer (see wait); a
    // request without an answer of its own counts as done once sent.
    request: (action: ManageAction) => {
      const confirms = confirmsManage(action);
      if (confirms) return wait({ via: "text", confirms }, () => ws.send(manageRequest(action)));
      if (ws.readyState !== WebSocket.OPEN) return Promise.reject(new Error("Not connected"));
      ws.send(manageRequest(action));
      return Promise.resolve();
    },
    // Other requests a control waits on: a text request (done on its
    // answer), Mumble messages (done once the state holds the change).
    ask: (body: string, confirms: (ev: ServerEvent) => boolean) => wait({ via: "text", confirms }, () => ws.send(body)),
    sendAwait: (messages: Bytes[], holds: (s: ServerState) => boolean) => wait({ via: "mumble", holds }, () => messages.forEach(send)),
    // Downloads a finished recording (staff): a link with a one-time token.
    // On another origin "download" does not count: a new tab, so an error
    // page does not replace the app. In the document for the click, so the
    // desktop app opens it in the browser (lib/externalLinks).
    downloadRecording: async (id: string) => {
      const a = document.createElement("a");
      a.href = recordingUrl(base, id, await askToken("recording-token"));
      a.download = "";
      a.target = "_blank";
      a.rel = "noopener";
      document.body.append(a);
      a.click();
      a.remove();
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
    // Our status and custom status (see lib/presence).
    setStatus: (status: Presence, custom?: CustomStatus) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(setStatusRequest(status, custom));
    },
    setAway: (on: boolean) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(awayRequest(on));
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
    requestBio: (user: number) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(bioRequest(user));
    },
    // Threads (see lib/threads).
    thread: (action: ThreadAction) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(threadRequest(action));
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
      const body = await upload(uploadTokenRequest(), "/api/upload", file, false);
      if (!body.key) throw new Error("upload failed");
      return body.key;
    },
    // Adds a custom emoji (admins and moderators); the list comes as an
    // event.
    uploadEmoji: async (name: string, file: File): Promise<void> => {
      await upload(emojiTokenRequest(name), "/api/emoji", file, true);
    },
    deleteEmoji: (name: string) =>
      wait({ via: "text", confirms: (ev) => ev.type === "emoji" && !ev.emoji.some((e) => e.name === name) }, () => ws.send(deleteEmojiRequest(name))),
    // Our personal emoji (see lib/personalEmoji): a new image, one from a
    // message (":name:" or "<:name:id>"), rename and delete; the list
    // comes as an event. Moderators remove an image everywhere.
    uploadUserEmoji: async (name: string, file: File): Promise<void> => {
      await upload(userEmojiTokenRequest(name), "/api/emoji/personal", file, true);
    },
    addUserEmoji: (emoji: string, name: string) =>
      wait({ via: "text", confirms: (ev) => ev.type === "my-emoji" && ev.emoji.some((e) => e.name === name) }, () =>
        ws.send(addUserEmojiRequest(emoji, name)),
      ),
    renameUserEmoji: (name: string, to: string) =>
      wait({ via: "text", confirms: (ev) => ev.type === "my-emoji" && ev.emoji.some((e) => e.name === to) }, () =>
        ws.send(renameUserEmojiRequest(name, to)),
      ),
    deleteUserEmoji: (name: string) =>
      wait({ via: "text", confirms: (ev) => ev.type === "my-emoji" && !ev.emoji.some((e) => e.name === name) }, () =>
        ws.send(deleteUserEmojiRequest(name)),
      ),
    removeEmojiImage: (emoji: string, id: number) =>
      wait({ via: "text", confirms: (ev) => ev.type === "removed-emoji" && ev.ids.includes(id) }, () => ws.send(removeEmojiImageRequest(emoji))),
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
