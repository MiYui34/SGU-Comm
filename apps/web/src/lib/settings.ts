import { DEFAULT_QUALITY, parseQuality, type Kind, type Quality } from "./stream/quality";
import type { NoiseMode } from "./voice/noise";
import type { TransmitMode } from "./voice/vad";

export type Settings = {
  mode: TransmitMode;
  pttKey: string; // KeyboardEvent.code
  vadThresholdDb: number;
  inputDeviceId: string | null;
  outputDeviceId: string | null;
  // Desktop notifications for mentions and private messages.
  notifications: boolean;
  // Close the microphone while muted (the browser stops showing it as
  // recording, but opening it again interrupts the sound for a moment).
  closeMicWhenMuted: boolean;
  // Others see a thumbnail of our screen share before they watch.
  streamPreviews: boolean;
  // See voice/noise.ts.
  noiseSuppression: NoiseMode;
  // Colours: "system" follows the system's light or dark setting.
  theme: ThemeSetting;
  // The browser's echo cancellation. Off with a headset: in WebKit (the
  // desktop app, Safari) on macOS it puts Bluetooth headsets into their
  // call mode, also with another microphone chosen.
  echoCancellation: boolean;
  // For music: none of the filters above, no automatic gain, more bits
  // and stereo (see voice/opus.ts).
  musicMode: boolean;
  // The camera to stream (null: the default), and the quality of each kind
  // of stream (also changed while live).
  cameraDeviceId: string | null;
  streamQuality: Record<Kind, Quality>;
};

// What to ask the browser for: exactly the chosen microphone (null: the
// default), with its own filters as set (browserNoise: its noise filter,
// when ours are off). Music: no filters, both channels if it has two.
export const micConstraints = (
  deviceId: string | null,
  o: { browserNoise: boolean; cancelEcho: boolean; music: boolean },
): MediaTrackConstraints => ({
  deviceId: deviceId ? { exact: deviceId } : undefined,
  echoCancellation: o.cancelEcho && !o.music,
  noiseSuppression: o.browserNoise && !o.music,
  autoGainControl: !o.music,
  ...(o.music && { channelCount: { ideal: 2 } }),
});

export type ThemeSetting = "dark" | "light" | "system";
export const themeOf = (setting: ThemeSetting, systemDark: boolean): "dark" | "light" =>
  setting === "system" ? (systemDark ? "dark" : "light") : setting;

export const METER_MIN_DB = -80;

export const DEFAULT_SETTINGS: Settings = {
  mode: "vad",
  pttKey: "Backquote",
  vadThresholdDb: -45,
  inputDeviceId: null,
  outputDeviceId: null,
  notifications: false,
  closeMicWhenMuted: false,
  streamPreviews: true,
  noiseSuppression: "browser",
  theme: "dark",
  echoCancellation: true,
  musicMode: false,
  cameraDeviceId: null,
  streamQuality: DEFAULT_QUALITY,
};

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n));
const str = (v: unknown) => (typeof v === "string" && v ? v : null);

// Stored data can be old or broken: keep what is valid, default the rest.
export const parseSettings = (raw: string | null): Settings => {
  let data: unknown;
  try {
    data = JSON.parse(raw ?? "null");
  } catch {
    return DEFAULT_SETTINGS;
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) return DEFAULT_SETTINGS;
  const d = data as Record<string, unknown>;
  const db = d.vadThresholdDb;
  const quality = (d.streamQuality ?? {}) as Record<string, unknown>;
  return {
    mode: d.mode === "vad" || d.mode === "ptt" ? d.mode : DEFAULT_SETTINGS.mode,
    pttKey: str(d.pttKey) ?? DEFAULT_SETTINGS.pttKey,
    vadThresholdDb:
      typeof db === "number" && db >= -120
        ? clamp(db, METER_MIN_DB, 0)
        : DEFAULT_SETTINGS.vadThresholdDb,
    inputDeviceId: str(d.inputDeviceId),
    outputDeviceId: str(d.outputDeviceId),
    notifications: d.notifications === true,
    closeMicWhenMuted: d.closeMicWhenMuted === true,
    streamPreviews: d.streamPreviews !== false,
    noiseSuppression: d.noiseSuppression === "strong" || d.noiseSuppression === "strongest" ? d.noiseSuppression : "browser",
    theme: d.theme === "light" || d.theme === "system" ? d.theme : "dark",
    echoCancellation: d.echoCancellation !== false,
    musicMode: d.musicMode === true,
    cameraDeviceId: str(d.cameraDeviceId),
    streamQuality: { screen: parseQuality(quality.screen, "screen"), camera: parseQuality(quality.camera, "camera") },
  };
};

// Push-to-talk works only while the page has focus (a browser limit).
// Typing in a text field never presses the key, but release always counts,
// so the key cannot get stuck.
export const pttAction = (
  type: "keydown" | "keyup",
  ev: { code: string; repeat: boolean; editable: boolean },
  pttKey: string,
): "down" | "up" | null => {
  if (ev.code !== pttKey) return null;
  if (type === "keyup") return "up";
  return ev.repeat || ev.editable ? null : "down";
};

// The PTT key must not also do its normal job, for example Space pressing a
// focused button (which happens on keyup). Text fields keep the key.
export const pttBlocksDefault = (ev: { code: string; repeat: boolean; editable: boolean }, pttKey: string) =>
  ev.code === pttKey && !ev.editable;

const KEY_NAMES: Record<string, string> = {
  Backquote: "`",
  Space: "Space",
  CapsLock: "Caps Lock",
};

export const keyLabel = (code: string): string => {
  if (KEY_NAMES[code]) return KEY_NAMES[code];
  const m = /^(Key|Digit)(.)$/.exec(code);
  if (m) return m[2]!;
  const side = /^(Shift|Control|Alt|Meta)(Left|Right)$/.exec(code);
  if (side) return `${side[2]} ${side[1]}`;
  return code;
};

export const meterPercent = (db: number) =>
  Math.round(clamp(((db - METER_MIN_DB) / -METER_MIN_DB) * 100, 0, 100));
