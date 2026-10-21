import { backgroundAudioWanted, voiceSession, type VoiceSessionAction } from "../mediaSession";

type Sinkable = { setSinkId?: (id: string) => Promise<void> };
export type VoiceSession = ReturnType<typeof voiceSession>;

// A client that goes away (a reconnect swaps it for a new one) keeps the
// session this long: the new one takes it over without a pause.
const RELEASE_MS = 1000;
const ACTIONS = ["pause", "stop", "hangup"] as const;

const showSession = (session: VoiceSession | null, run: (action: VoiceSessionAction) => void) => {
  const media = navigator.mediaSession;
  media.metadata = session && new MediaMetadata(session.metadata);
  media.playbackState = session ? "playing" : "none";
  for (const action of ACTIONS) {
    const what = session?.actions[action];
    try {
      media.setActionHandler(action as MediaSessionAction, what ? () => run(what) : null);
    } catch {
      // Not every browser knows every action.
    }
  }
};

// Where the voice mix goes: the speakers, or while in voice on Android
// an <audio> element with a Media Session (see mediaSession.ts).
// The element only once it plays: a refused play() (no user gesture yet)
// keeps the direct path, so voice never goes silent, and never both.
const createVoiceOutput = (ctx: AudioContext, background: boolean) => {
  const input = ctx.createGain();
  input.connect(ctx.destination);
  const element = background ? new Audio() : null;
  let direct = true;
  let active = false;
  let closed = false;
  let release: ReturnType<typeof setTimeout> | undefined;
  const route = (toElement: boolean) => {
    if (direct !== toElement) return;
    direct = !toElement;
    if (direct) input.connect(ctx.destination);
    else input.disconnect(ctx.destination);
  };
  // Refused or stopped from outside (another app took the audio): tried
  // again on the next touch.
  const play = () => void (active && element?.paused && element.play().catch(() => {}));
  if (element) {
    const stream = ctx.createMediaStreamDestination();
    input.connect(stream);
    element.srcObject = stream.stream;
    element.onplaying = () => route(true);
    element.onpause = () => route(false);
  }
  const setActive = (on: boolean) => {
    if (!element || on === active) return;
    active = on;
    if (on) document.addEventListener("pointerdown", play);
    else document.removeEventListener("pointerdown", play);
    if (on) play();
    else element.pause();
  };

  return {
    input: input as AudioNode,
    // null: out of voice. Does nothing off Android.
    setVoice: (session: VoiceSession | null, run: (action: VoiceSessionAction) => void) => {
      if (!element || closed) return;
      clearTimeout(release);
      showSession(session, run);
      setActive(session !== null);
    },
    // The client went away: out of voice, unless a new one takes over.
    release: () => {
      if (closed) return;
      clearTimeout(release);
      release = setTimeout(() => (showSession(null, () => {}), setActive(false)), RELEASE_MS);
    },
    setSinkId: (deviceId: string | null) => {
      for (const sink of [ctx, element] as (Sinkable | null)[]) void sink?.setSinkId?.(deviceId ?? "").catch((err) => console.warn("output device", err));
    },
    // The context goes (see closeVoiceOutput). The media session stays:
    // the next context's output takes it over: later calls (the old
    // client's teardown) do nothing.
    close: () => {
      closed = true;
      clearTimeout(release);
      if (element) element.onplaying = element.onpause = null;
      setActive(false);
      if (element) element.srcObject = null;
      input.disconnect();
    },
  };
};

export type VoiceOutput = ReturnType<typeof createVoiceOutput>;

// One per audio context: it outlives connections (reconnects reuse it).
const outputs = new WeakMap<AudioContext, VoiceOutput>();
export const voiceOutput = (ctx: AudioContext): VoiceOutput => {
  let out = outputs.get(ctx);
  if (!out) {
    const background = backgroundAudioWanted({
      android: /Android/i.test(navigator.userAgent),
      mediaSession: "mediaSession" in navigator && typeof MediaMetadata !== "undefined",
      mediaStreams: "createMediaStreamDestination" in ctx,
    });
    outputs.set(ctx, (out = createVoiceOutput(ctx, background)));
  }
  return out;
};

// Before the context closes (an audio restart, a new login): its element
// stops, and no late release ends the next output's media session.
export const closeVoiceOutput = (ctx: AudioContext) => {
  outputs.get(ctx)?.close();
  outputs.delete(ctx);
};
