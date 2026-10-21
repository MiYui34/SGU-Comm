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

// Whether the page shows, and a call when it shows again (see documentPage).
export type Page = { visible: () => boolean; onVisible: (listener: () => void) => () => void };
const alwaysVisible: Page = { visible: () => true, onVisible: () => () => {} };
export const documentPage = (doc: Document): Page => ({
  visible: () => doc.visibilityState === "visible",
  onVisible: (listener) => {
    const l = () => doc.visibilityState === "visible" && listener();
    doc.addEventListener("visibilitychange", l);
    return () => doc.removeEventListener("visibilitychange", l);
  },
});

// Calls back when the outputs changed: once after a burst of devicechange
// events (they come several at a time). Not for microphones. Only while
// the page shows: a phone in a pocket keeps its voice (a new context there
// may not start without a tap); a change meanwhile counts when it shows
// again, compared with the list of the current context. Returns stop.
export const watchOutputDevices = (
  media: Media,
  changed: () => void,
  { debounceMs = 500, page = alwaysVisible }: { debounceMs?: number; page?: Page } = {},
): (() => void) => {
  let last: string | null = null; // the outputs the current context knows
  let stopped = false;
  let hiddenChange = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let reads = 0; // a slow read must not win over a newer one
  const read = () => media.enumerateDevices().then(outputSignature, () => null);
  void read().then((s) => (last ??= s));
  const check = async () => {
    const mine = ++reads;
    const now = await read();
    if (stopped || mine !== reads || now === null) return;
    if (!page.visible()) return void (hiddenChange = true);
    const was = last;
    last = now;
    if (was !== null && was !== now) changed();
  };
  const onChange = () => {
    clearTimeout(timer);
    timer = setTimeout(() => void check(), debounceMs);
  };
  const stopPage = page.onVisible(() => {
    if (!hiddenChange) return;
    hiddenChange = false;
    void check();
  });
  media.addEventListener("devicechange", onChange);
  return () => {
    stopped = true;
    clearTimeout(timer);
    stopPage();
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
