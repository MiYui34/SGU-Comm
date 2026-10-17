// Long press on touch screens: phones have no right-click. Holding still
// for DELAY_MS opens what right-click opens; moving (scrolling) or letting
// go earlier does not.
const DELAY_MS = 500;
const TOLERANCE_PX = 10;

export const createLongPress = (onLongPress: (x: number, y: number) => void) => {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let origin = { x: 0, y: 0 };
  let fired = false;
  const cancel = () => {
    clearTimeout(timer);
    timer = undefined;
  };
  return {
    start: (x: number, y: number) => {
      cancel();
      fired = false;
      origin = { x, y };
      timer = setTimeout(() => {
        timer = undefined;
        fired = true;
        onLongPress(x, y);
      }, DELAY_MS);
    },
    move: (x: number, y: number) => {
      if (Math.hypot(x - origin.x, y - origin.y) > TOLERANCE_PX) cancel();
    },
    // The browser opened its own context menu (Android): nothing to do.
    native: cancel,
    // true: it fired, so the click that follows must not count.
    end: () => {
      cancel();
      return fired;
    },
  };
};

// Touch listeners for a long press on `node`. onNative: the browser's own
// long-press menu (Android: a trusted contextmenu event). skip: presses
// that start there do not count.
const attach = (
  node: HTMLElement,
  press: ReturnType<typeof createLongPress>,
  onNative: (e: Event) => void,
  skip?: string,
) => {
  const touch = (e: TouchEvent) => e.touches[0] ?? e.changedTouches[0];
  const skipped = (e: Event) => !!skip && e.target instanceof Element && !!e.target.closest(skip);
  const start = (e: TouchEvent) => {
    if (!skipped(e)) press.start(touch(e)!.clientX, touch(e)!.clientY);
  };
  const move = (e: TouchEvent) => press.move(touch(e)!.clientX, touch(e)!.clientY);
  const end = (e: TouchEvent) => {
    if (press.end()) e.preventDefault(); // no click after the menu opened
  };
  const native = (e: Event) => {
    if (e.isTrusted && !skipped(e)) onNative(e);
  };
  node.addEventListener("touchstart", start, { passive: true });
  node.addEventListener("touchmove", move, { passive: true });
  node.addEventListener("touchend", end);
  node.addEventListener("touchcancel", end);
  node.addEventListener("contextmenu", native);
  return {
    destroy() {
      press.end();
      node.removeEventListener("touchstart", start);
      node.removeEventListener("touchmove", move);
      node.removeEventListener("touchend", end);
      node.removeEventListener("touchcancel", end);
      node.removeEventListener("contextmenu", native);
    },
  };
};

// A Svelte action: a long press sends a contextmenu event to the element,
// so its right-click handler runs.
export const longPress = (node: HTMLElement) => {
  const press = createLongPress((x, y) => {
    node.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: x, clientY: y, detail: 1 }));
  });
  return attach(node, press, press.native);
};

// Text fields and media controls keep their own long press (caret, seeking).
const OWN_LONG_PRESS = "input, textarea, select, video, audio, [contenteditable]";

// A Svelte action for touch screens: a long press runs onHold. The
// browser's own long-press menu (Android) runs it too, instead of its menu;
// a right-click with a mouse stays as it is.
export const hold = (node: HTMLElement, onHold: () => void) => {
  let run = onHold;
  const press = createLongPress(() => run());
  const touchScreen = () => matchMedia("(hover: none)").matches;
  const listeners = attach(
    node,
    press,
    (e) => {
      press.native();
      if (!touchScreen()) return;
      e.preventDefault();
      run();
    },
    OWN_LONG_PRESS,
  );
  return {
    update(next: () => void) {
      run = next;
    },
    destroy: listeners.destroy,
  };
};
