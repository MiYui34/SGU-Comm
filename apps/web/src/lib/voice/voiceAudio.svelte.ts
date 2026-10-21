type Closable = { close: () => void };

// A connection's playback and capture, on one audio context. An audio
// restart (see restart.ts) moves both to a new one: the UI sees new
// objects and applies its settings to them again (volumes, devices, the
// microphone). null context: no voice support.
export const createVoiceAudio = <P extends Closable, C extends Closable>(
  ctx: AudioContext | null,
  make: (ctx: AudioContext) => { playback: P; capture: C },
) => {
  let parts = $state.raw(ctx && make(ctx));
  let closed = false;
  const closeParts = () => {
    parts?.capture.close();
    parts?.playback.close();
  };
  return {
    get playback(): P | null {
      return parts?.playback ?? null;
    },
    get capture(): C | null {
      return parts?.capture ?? null;
    },
    move: (next: AudioContext) => {
      if (closed || !parts) return;
      closeParts();
      parts = make(next);
    },
    close: () => {
      closed = true;
      closeParts();
    },
  };
};
