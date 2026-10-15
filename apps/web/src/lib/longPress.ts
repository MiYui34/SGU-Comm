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

// A Svelte action: a long press sends a contextmenu event to the element,
// so its right-click handler runs.
export const longPress = (node: HTMLElement) => {
  const press = createLongPress((x, y) => {
    node.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: x, clientY: y, detail: 1 }));
  });
  const touch = (e: TouchEvent) => e.touches[0] ?? e.changedTouches[0];
  const start = (e: TouchEvent) => press.start(touch(e)!.clientX, touch(e)!.clientY);
  const move = (e: TouchEvent) => press.move(touch(e)!.clientX, touch(e)!.clientY);
  const end = (e: TouchEvent) => {
    if (press.end()) e.preventDefault(); // no click after the menu opened
  };
  const native = (e: Event) => {
    if (e.isTrusted) press.native();
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
