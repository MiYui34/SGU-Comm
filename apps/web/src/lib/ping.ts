// A short, soft two-note sound for mentions (browser only), on the chosen
// output device, at most once a second.
let ctx: AudioContext | null = null;
let last = 0;

export const playPing = (outputDeviceId: string | null) => {
  const now = performance.now();
  if (now - last < 1000) return;
  last = now;
  try {
    ctx ??= new AudioContext();
    const sink = ctx as AudioContext & { setSinkId?: (id: string) => Promise<void> };
    void sink.setSinkId?.(outputDeviceId ?? "").catch(() => {});
    const t = ctx.currentTime;
    for (const [i, freq] of [880, 1320].entries()) {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.frequency.value = freq;
      gain.gain.setValueAtTime(0.0001, t + i * 0.09);
      gain.gain.exponentialRampToValueAtTime(0.15, t + i * 0.09 + 0.01);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + i * 0.09 + 0.18);
      osc.connect(gain).connect(ctx.destination);
      osc.start(t + i * 0.09);
      osc.stop(t + i * 0.09 + 0.2);
    }
  } catch {
    // No audio: no sound.
  }
};
