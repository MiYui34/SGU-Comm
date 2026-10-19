// Short, soft notes (browser only) on the chosen output device: the mention
// ping, and the voice sounds (sounds.ts).
let ctx: AudioContext | null = null;
let last = 0;

// Each note starts `step` seconds after the one before; peak: its loudness.
export const playNotes = (outputDeviceId: string | null, notes: readonly number[], step: number, peak: number) => {
  if (peak <= 0) return;
  try {
    ctx ??= new AudioContext();
    void ctx.resume().catch(() => {});
    const sink = ctx as AudioContext & { setSinkId?: (id: string) => Promise<void> };
    void sink.setSinkId?.(outputDeviceId ?? "").catch(() => {});
    const t = ctx.currentTime;
    for (const [i, freq] of notes.entries()) {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.frequency.value = freq;
      gain.gain.setValueAtTime(0.0001, t + i * step);
      gain.gain.exponentialRampToValueAtTime(peak, t + i * step + 0.01);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + i * step + 0.18);
      osc.connect(gain).connect(ctx.destination);
      osc.start(t + i * step);
      osc.stop(t + i * step + 0.2);
    }
  } catch {
    // No audio: no sound.
  }
};

// Mentions: two notes, at most once a second.
export const playPing = (outputDeviceId: string | null) => {
  const now = performance.now();
  if (now - last < 1000) return;
  last = now;
  playNotes(outputDeviceId, [880, 1320], 0.09, 0.15);
};
