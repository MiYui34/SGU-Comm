import {
  addOwnMessage,
  audioFormat,
  createMessageStream,
  decodeAudio,
  encodeAudio,
  encodeVoicePacket,
  handshake,
  initialState,
  ping,
  reduce,
  sendText,
  type Bytes,
  type MumbleMessage,
  type OutgoingAudio,
  type ServerState,
  type TextTarget,
} from "@kallisti/protocol";
import { createCapture } from "./voice/capture.svelte";
import { createPlayback } from "./voice/playback.svelte";

// The server drops clients after 30 s without a ping.
const PING_INTERVAL_MS = 15_000;

export type Credentials = { username: string; password?: string };

export type Connection = ReturnType<typeof connect>;

// Thin effectful shell around the pure protocol code. Owns the audio
// context (null: no voice support), so voice lives and dies with the socket.
export const connect = (url: string, credentials: Credentials, audio: AudioContext | null) => {
  let state = $state.raw<ServerState>(initialState);
  let closed = $state(false);
  const push = createMessageStream();
  const ws = new WebSocket(url);
  ws.binaryType = "arraybuffer";
  let timer: ReturnType<typeof setInterval> | undefined;

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
    try {
      const messages = push(new Uint8Array(event.data as ArrayBuffer));
      messages.forEach(routeAudio);
      state = messages.reduce((s, m) => reduce(s, m, Date.now()), state);
    } catch (err) {
      // A broken frame leaves the stream out of sync: give up on it.
      console.error("protocol error:", err);
      ws.close();
    }
  };
  ws.onclose = () => {
    clearInterval(timer);
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
