// Local, per-user playback settings, keyed by user name so they survive
// reconnects (sessions change, names usually do not).
export type UserAudio = { volume: number; muted: boolean };
export type UserAudioPrefs = Readonly<Record<string, UserAudio>>;

export const DEFAULT_USER_AUDIO: UserAudio = { volume: 1, muted: false };
export const MAX_VOLUME = 2;

const clampVolume = (v: number) => Math.min(MAX_VOLUME, Math.max(0, v));

const parseEntry = (v: unknown): UserAudio | null => {
  if (!v || typeof v !== "object" || Array.isArray(v)) return null;
  const e = v as Record<string, unknown>;
  return {
    volume: typeof e.volume === "number" ? clampVolume(e.volume) : DEFAULT_USER_AUDIO.volume,
    muted: typeof e.muted === "boolean" ? e.muted : DEFAULT_USER_AUDIO.muted,
  };
};

export const parseUserAudio = (raw: string | null): UserAudioPrefs => {
  let data: unknown;
  try {
    data = JSON.parse(raw ?? "null");
  } catch {
    return {};
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) return {};
  return Object.fromEntries(
    Object.entries(data).flatMap(([name, v]) => {
      const entry = parseEntry(v);
      return entry ? [[name, entry]] : [];
    }),
  );
};

export const userAudioFor = (prefs: UserAudioPrefs, name: string): UserAudio =>
  prefs[name] ?? DEFAULT_USER_AUDIO;

export const updateUserAudio = (
  prefs: UserAudioPrefs,
  name: string,
  patch: Partial<UserAudio>,
): UserAudioPrefs => {
  const next = { ...userAudioFor(prefs, name), ...patch };
  next.volume = clampVolume(next.volume);
  const { [name]: _, ...rest } = prefs;
  const isDefault =
    next.volume === DEFAULT_USER_AUDIO.volume && next.muted === DEFAULT_USER_AUDIO.muted;
  return isDefault ? rest : { ...rest, [name]: next };
};

export const gainOf = ({ volume, muted }: UserAudio) => (muted ? 0 : volume);
