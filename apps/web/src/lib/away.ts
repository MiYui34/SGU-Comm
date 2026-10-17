import { isAway } from "./presence";

const CHECK_MS = 15_000;
const INPUT = ["keydown", "pointerdown", "pointermove", "wheel", "touchstart"] as const;

// Watches keyboard and pointer input and page visibility; onChange gets
// true when the user is away (see isAway) and false on the next input.
// Returns a stop function.
export const watchAway = (onChange: (away: boolean) => void): (() => void) => {
  let lastInput = Date.now();
  let hiddenSince: number | null = document.hidden ? Date.now() : null;
  let away = false;
  const set = (now: boolean) => {
    if (now !== away) onChange((away = now));
  };
  const check = () => set(isAway({ lastInput, hiddenSince, now: Date.now() }));
  const input = () => {
    lastInput = Date.now();
    if (away) check();
  };
  const visibility = () => {
    hiddenSince = document.hidden ? Date.now() : null;
    check();
  };
  INPUT.forEach((e) => window.addEventListener(e, input, { passive: true, capture: true }));
  document.addEventListener("visibilitychange", visibility);
  const timer = setInterval(check, CHECK_MS);
  return () => {
    INPUT.forEach((e) => window.removeEventListener(e, input, { capture: true }));
    document.removeEventListener("visibilitychange", visibility);
    clearInterval(timer);
  };
};
