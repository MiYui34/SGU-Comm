import type { AudioPacket } from "@kallisti/protocol";
import { SvelteSet } from "svelte/reactivity";
import { createLimiter } from "./limiter";
import { playoutFor, type PlaybackBuffer } from "./buffer";
import { chunkOutcome, nextStart, startsAnew, type PlayoutConfig } from "./schedule";
import type { Counters } from "../connectionQuality";

const SAMPLE_RATE = 48_000;
// Mumble counts frames in 10 ms steps.
const FRAME_US = 10_000;
const SPEAKING_TIMEOUT_MS = 250;

type Speaker = {
  decoder: AudioDecoder;
  gain: GainNode;
  end: number | null;
  // The playout timing of the current stream (see playoutFor).
  cfg: PlayoutConfig;
  // Where the stream goes on (µs, null: a new talk spurt), and for each
  // chunk in the decoder (outputs come in order) its frame time and whether
  // it ends a spurt (see chunkOutcome). Not the output's timestamp: a
  // decoder may count those itself.
  next: number | null;
  queued: { at: number; end: boolean }[];
  timer?: ReturnType<typeof setTimeout>;
};

export type Playback = ReturnType<typeof createPlayback>;

export const createPlayback = (ctx: AudioContext) => {
  const speakers = new Map<number, Speaker>();
  // Per-user volume (0 = locally muted). Kept apart from `speakers`, which
  // come and go with each talk spurt's decoder.
  const gains = new Map<number, number>();
  const speaking = new SvelteSet<number>();
  const output = ctx.createGain();
  const limiter = createLimiter(ctx);
  output.connect(limiter.input);
  limiter.output.connect(ctx.destination);
  let deaf = false;
  let delaySetting: PlaybackBuffer = "low";
  // For the connection info (counters only: this is the audio path).
  const counters: Counters = { good: 0, late: 0, lost: 0 };

  const play = (speaker: Speaker, data: AudioData) => {
    const now = ctx.currentTime * SAMPLE_RATE;
    // A changed buffer applies from the next start: never cut a stream.
    if (startsAnew(speaker.end, now)) speaker.cfg = playoutFor(delaySetting, false);
    const start = nextStart(speaker.end, now, speaker.cfg);
    const duration = (data.numberOfFrames / SAMPLE_RATE) * 1e6;
    const chunk = speaker.queued.shift() ?? { at: data.timestamp, end: false };
    const { late, lost } = chunkOutcome({ expected: speaker.next, timestamp: chunk.at, duration, start, prevEnd: speaker.end, now });
    counters[late ? "late" : "good"]++;
    counters.lost += lost;
    speaker.next = chunk.end ? null : chunk.at + duration;
    if (start === null) return data.close();
    // Stereo streams (music bots) keep both channels; voice is mono in
    // both.
    const buffer = ctx.createBuffer(data.numberOfChannels, data.numberOfFrames, data.sampleRate);
    for (let ch = 0; ch < data.numberOfChannels; ch++)
      data.copyTo(buffer.getChannelData(ch), { planeIndex: ch, format: "f32-planar" });
    const source = ctx.createBufferSource();
    source.buffer = buffer;
    source.connect(speaker.gain);
    source.start(start / SAMPLE_RATE);
    speaker.end = start + data.numberOfFrames;
    data.close();
  };

  const remove = (session: number) => {
    const speaker = speakers.get(session);
    if (!speaker) return;
    clearTimeout(speaker.timer);
    if (speaker.decoder.state !== "closed") speaker.decoder.close();
    speaker.gain.disconnect();
    speakers.delete(session);
    speaking.delete(session);
  };

  const speakerFor = (session: number): Speaker => {
    const existing = speakers.get(session);
    if (existing) return existing;
    const gain = ctx.createGain();
    gain.gain.value = gains.get(session) ?? 1;
    gain.connect(output);
    const speaker: Speaker = {
      gain,
      end: null,
      cfg: playoutFor(delaySetting, false),
      next: null,
      queued: [],
      decoder: new AudioDecoder({
        output: (data) => play(speaker, data),
        // A broken stream: drop the decoder, the next packet makes a new one.
        error: (err) => (console.warn("decoder error", session, err), remove(session)),
      }),
    };
    // Two channels: stereo packets keep their image, mono ones come out the
    // same on both sides.
    speaker.decoder.configure({ codec: "opus", sampleRate: SAMPLE_RATE, numberOfChannels: 2 });
    speakers.set(session, speaker);
    return speaker;
  };

  const markSpeaking = (session: number, speaker: Speaker, terminator: boolean) => {
    clearTimeout(speaker.timer);
    if (terminator) return speaking.delete(session);
    speaking.add(session);
    speaker.timer = setTimeout(() => speaking.delete(session), SPEAKING_TIMEOUT_MS);
  };

  return {
    speaking: speaking as ReadonlySet<number>,
    // Must not throw: it runs inside the control connection's message handler.
    handle: (packet: AudioPacket) => {
      if (deaf) return;
      try {
        const speaker = speakerFor(packet.session);
        markSpeaking(packet.session, speaker, packet.terminator);
        // Locally muted: keep the speaking ring, skip the decoding work.
        if (gains.get(packet.session) === 0) return;
        const at = packet.frameNumber * FRAME_US;
        if (packet.opus.length === 0) {
          // A bare terminator ends the spurt after the chunks in the decoder.
          const last = speaker.queued.at(-1);
          if (packet.terminator && last) last.end = true;
          else if (packet.terminator) speaker.next = null;
          return;
        }
        speaker.queued.push({ at, end: packet.terminator });
        speaker.decoder.decode(new EncodedAudioChunk({ type: "key", timestamp: at, data: packet.opus }));
      } catch (err) {
        // For example a decoder that failed and is closed, before its error
        // callback ran. Drop it, the next packet makes a new one.
        console.warn("voice packet dropped", packet.session, err);
        remove(packet.session);
      }
    },
    remove,
    stats: (): Counters => ({ ...counters }),
    // The user left: also drop their volume.
    forget: (session: number) => {
      remove(session);
      gains.delete(session);
    },
    setUserGain: (session: number, value: number) => {
      // Unmuted: the decoder skipped packets meanwhile, so start a new one.
      if (gains.get(session) === 0 && value > 0) remove(session);
      gains.set(session, value);
      const speaker = speakers.get(session);
      // A short ramp: a jump in gain can click.
      speaker?.gain.gain.setTargetAtTime(value, ctx.currentTime, 0.02);
    },
    // Does nothing where outputDeviceSupported() is false.
    setOutputDevice: (deviceId: string | null) => {
      const sink = ctx as AudioContext & { setSinkId?: (id: string) => Promise<void> };
      void sink.setSinkId?.(deviceId ?? "").catch((err) => console.warn("output device", err));
    },
    setBuffer: (value: PlaybackBuffer) => {
      delaySetting = value;
    },
    setDeaf: (value: boolean) => {
      deaf = value;
      // Also silences chunks that are already scheduled.
      output.gain.value = deaf ? 0 : 1;
      if (deaf) [...speakers.keys()].forEach(remove);
    },
    close: () => [...speakers.keys()].forEach(remove),
  };
};
