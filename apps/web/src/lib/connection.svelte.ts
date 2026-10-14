import {
  addOwnMessage,
  createMessageStream,
  handshake,
  initialState,
  ping,
  reduce,
  sendText,
  type Bytes,
  type ServerState,
} from "@mumble-discord/protocol";

// The server drops clients after 30 s without a ping.
const PING_INTERVAL_MS = 15_000;

export type Credentials = { username: string; password?: string };

export type Connection = ReturnType<typeof connect>;

// Thin effectful shell around the pure protocol code.
export const connect = (url: string, credentials: Credentials) => {
  let state = $state.raw<ServerState>(initialState);
  let closed = $state(false);
  const push = createMessageStream();
  const ws = new WebSocket(url);
  ws.binaryType = "arraybuffer";
  let timer: ReturnType<typeof setInterval> | undefined;

  ws.onopen = () => {
    ws.send(handshake(credentials));
    timer = setInterval(() => ws.send(ping(Date.now())), PING_INTERVAL_MS);
  };
  ws.onmessage = (event) => {
    try {
      const messages = push(new Uint8Array(event.data as ArrayBuffer));
      state = messages.reduce((s, m) => reduce(s, m, Date.now()), state);
    } catch (err) {
      // A broken frame leaves the stream out of sync: give up on it.
      console.error("protocol error:", err);
      ws.close();
    }
  };
  ws.onclose = () => {
    clearInterval(timer);
    closed = true;
  };

  const send = (bytes: Bytes) => {
    if (ws.readyState === WebSocket.OPEN) ws.send(bytes);
  };

  return {
    get state() {
      return state;
    },
    get closed() {
      return closed;
    },
    send,
    sendText: (channelId: number, text: string) => {
      send(sendText(channelId, text));
      state = addOwnMessage(state, channelId, text, Date.now());
    },
    close: () => ws.close(),
  };
};
