// A connection's playback and capture, on one audio context. An audio
// restart (see restart.ts) moves both to a new one: the UI sees new
// objects and applies its settings to them again (volumes, devices, the
// microphone). null context: no voice support.
// M: the open microphone a capture hands to the next (see capture.svelte.ts).
export const createVoiceAudio = <P extends { close: () => void }, M, C extends { close: (keepMic?: boolean) => M | null }>(
  ctx: AudioContext | null,
  make: (ctx: AudioContext, mic: M | null) => { playback: P; capture: C },
) => {
  let parts = $state.raw(ctx && make(ctx, null));
  let closed = false;
  // keepMic: the microphone stays open for the new capture.
  const closeParts = (keepMic = false) => {
    const mic = parts?.capture.close(keepMic) ?? null;
    parts?.playback.close();
    return mic;
  };
  return {
    get playback(): P | null {
      return parts?.playback ?? null;
    },
    get capture(): C | null {
      return parts?.capture ?? null;
    },
    move: (next: AudioContext, keepMic = false) => {
      if (closed || !parts) return;
      parts = make(next, closeParts(keepMic));
    },
    close: () => {
      closed = true;
      closeParts();
    },
  };
};
