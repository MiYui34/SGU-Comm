const FLOOR_DB = -100;

export const levelDb = (samples: Float32Array): number => {
  let sum = 0;
  for (const s of samples) sum += s * s;
  const rms = Math.sqrt(sum / samples.length);
  return rms > 0 ? Math.max(FLOOR_DB, 20 * Math.log10(rms)) : FLOOR_DB;
};

export type VadState = { talking: boolean; silentMs: number };
export type VadConfig = { thresholdDb: number; holdMs: number };

// Voice activity with a hold time, so short pauses do not cut words.
export const vadStep = (
  state: VadState,
  db: number,
  frameMs: number,
  cfg: VadConfig,
): VadState => {
  if (db >= cfg.thresholdDb) return { talking: true, silentMs: 0 };
  const silentMs = state.silentMs + frameMs;
  return { talking: state.talking && silentMs < cfg.holdMs, silentMs };
};

export type TransmitMode = "vad" | "ptt";

export const shouldTransmit = (s: {
  mode: TransmitMode;
  vadTalking: boolean;
  pttDown: boolean;
  muted: boolean;
}): boolean => !s.muted && (s.mode === "vad" ? s.vadTalking : s.pttDown);
