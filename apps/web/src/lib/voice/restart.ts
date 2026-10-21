// A fresh audio context when voice starts and when the output device
// changes: Chrome resamples a context made for another device, and music
// then crackles (see meta/issues/restart-audio.md).

type Device = Pick<MediaDeviceInfo, "kind" | "deviceId" | "groupId" | "label">;

// The outputs, in a form to compare: the default entry's label and group
// change when the system's default device changes.
export const outputSignature = (devices: readonly Device[]): string =>
  devices
    .filter((d) => d.kind === "audiooutput")
    .map((d) => `${d.deviceId}|${d.groupId}|${d.label}`)
    .sort()
    .join("\n");

// The user joins a voice channel from outside voice (the lobby): a click,
// so the new context may start. Moves between voice channels keep theirs.
export const restartOnJoin = (s: { inVoice: boolean; lobby: boolean; target: number }): boolean =>
  !s.inVoice && !(s.lobby && s.target === 0);

type Media = Pick<MediaDevices, "enumerateDevices"> & {
  addEventListener: (type: "devicechange", listener: () => void) => void;
  removeEventListener: (type: "devicechange", listener: () => void) => void;
};

// Calls back when the outputs changed: once after a burst of devicechange
// events (they come several at a time). Not for microphones. Returns stop.
export const watchOutputDevices = (media: Media, changed: () => void, debounceMs = 500): (() => void) => {
  let last: string | null = null;
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let reads = 0; // a slow read must not win over a newer one
  const read = () => media.enumerateDevices().then(outputSignature, () => null);
  void read().then((s) => (last ??= s));
  const check = async () => {
    const mine = ++reads;
    const now = await read();
    if (stopped || mine !== reads || now === null) return;
    const was = last;
    last = now;
    if (was !== null && was !== now) changed();
  };
  const onChange = () => {
    clearTimeout(timer);
    timer = setTimeout(() => void check(), debounceMs);
  };
  media.addEventListener("devicechange", onChange);
  return () => {
    stopped = true;
    clearTimeout(timer);
    media.removeEventListener("devicechange", onChange);
  };
};

// The new context first (in a click, while the gesture counts), everything
// moved to it, then the old one closed. null: no voice support.
export const swapContext = <C>(old: C | null, make: () => C, move: (ctx: C) => void, close: (ctx: C) => void): C | null => {
  if (old === null) return null;
  const next = make();
  move(next);
  close(old);
  return next;
};
