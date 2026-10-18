import type { AudioPacket } from "@kallisti/protocol";
import { SvelteSet } from "svelte/reactivity";
import { createLimiter } from "./limiter";
import { chunkOutcome, nextStart, type PlayoutConfig } from "./schedule";
import type { Counters } from "../connectionQuality";

const SAMPLE_RATE = 48_000;
// Mumble counts frames in 10 ms steps.
const FRAME_US = 10_000;
const SPEAKING_TIMEOUT_MS = 250;
// 60 ms start delay, 300 ms maximum queue, in samples.
const PLAYOUT: PlayoutConfig = { startDelay: 2880, maxLatency: 14_400 };

type Speaker = {
  decoder: AudioDecoder;
  gain: GainNode;
  end: number | null;
  // Where the stream goes on (µs, null: a new talk spurt), and for each
  // chunk in the decoder, whether it ends a spurt (see chunkOutcome).
  next: number | null;
  ends: boolean[];
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
  // For the connection info (counters only: this is the audio path).
  const counters: Counters = { good: 0, late: 0, lost: 0 };

  const play = (speaker: Speaker, data: AudioData) => {
    const now = ctx.currentTime * SAMPLE_RATE;
    const start = nextStart(speaker.end, now, PLAYOUT);
    const duration = (data.numberOfFrames / SAMPLE_RATE) * 1e6;
    const { late, lost } = chunkOutcome({ expected: speaker.next, timestamp: data.timestamp, duration, start, prevEnd: speaker.end, now });
    counters[late ? "late" : "good"]++;
    counters.lost += lost;
    speaker.next = speaker.ends.shift() ? null : data.timestamp + duration;
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
      next: null,
      ends: [],
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
        if (packet.opus.length === 0) {
          // A bare terminator ends the spurt after the chunks in the decoder.
          if (packet.terminator && !speaker.ends.length) speaker.next = null;
          else if (packet.terminator) speaker.ends[speaker.ends.length - 1] = true;
          return;
        }
        speaker.ends.push(packet.terminator);
        speaker.decoder.decode(
          new EncodedAudioChunk({
            type: "key",
            timestamp: packet.frameNumber * FRAME_US,
            data: packet.opus,
          }),
        );
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
    setDeaf: (value: boolean) => {
      deaf = value;
      // Also silences chunks that are already scheduled.
      output.gain.value = deaf ? 0 : 1;
      if (deaf) [...speakers.keys()].forEach(remove);
    },
    close: () => [...speakers.keys()].forEach(remove),
  };
};
