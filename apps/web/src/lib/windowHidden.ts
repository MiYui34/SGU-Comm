// Is the window hidden (page hidden, or the desktop window minimised)? Then
// animated content need not paint (app.css: html[data-window-hidden]). The
// desktop app keeps painting in the background (backgroundThrottling
// "disabled" in tauri.conf.json, for voice), and WebKitGTK may keep a
// minimised window "visible". Tauri has no minimise event: the window is
// asked on resize and focus changes, and every POLL_MS while it has no focus.
// Wayland does not tell apps that they are minimised: there only the page's
// visibility counts.

export const POLL_MS = 2000;

// The parts of Tauri's window used here (getCurrentWindow()).
export type WindowApi = {
  isMinimized: () => Promise<boolean>;
  isVisible: () => Promise<boolean>;
  onResized: (cb: () => void) => Promise<() => void>;
  onFocusChanged: (cb: (e: { payload: boolean }) => void) => Promise<() => void>;
};

type Doc = Pick<Document, "visibilityState" | "hasFocus" | "addEventListener" | "removeEventListener">;

// onChange gets each change of the hidden state (also a hidden start).
// Returns a stop function.
export const watchWindowHidden = (onChange: (hidden: boolean) => void, { doc, win }: { doc: Doc; win?: WindowApi | null }) => {
  let hidden = false;
  let minimized = false;
  let stopped = false;
  let timer: ReturnType<typeof setInterval> | undefined;
  const unlisten: (() => void)[] = [];
  const update = () => {
    const now = doc.visibilityState === "hidden" || minimized;
    if (now !== hidden) onChange((hidden = now));
  };
  const setMinimized = (now: boolean) => {
    minimized = now;
    update();
  };

  const watchWindow = (win: WindowApi) => {
    let asks = 0;
    const ask = async () => {
      const mine = ++asks;
      const now = await Promise.all([win.isMinimized(), win.isVisible()]).then(([min, visible]) => min || !visible, () => false);
      if (mine === asks && !stopped) setMinimized(now);
    };
    const focus = (focused: boolean) => {
      clearInterval(timer);
      timer = undefined;
      if (stopped) return;
      if (focused) {
        asks++; // a focused window is shown: older answers do not count
        setMinimized(false);
      } else {
        void ask();
        timer = setInterval(() => void ask(), POLL_MS);
      }
    };
    const keep = (p: Promise<() => void>) => void p.then((off) => (stopped ? off() : unlisten.push(off)), () => {});
    keep(win.onResized(() => void ask()));
    keep(win.onFocusChanged((e) => focus(e.payload)));
    focus(doc.hasFocus());
  };

  doc.addEventListener("visibilitychange", update);
  update();
  if (win) watchWindow(win);
  return () => {
    stopped = true;
    doc.removeEventListener("visibilitychange", update);
    clearInterval(timer);
    unlisten.forEach((off) => off());
  };
};
