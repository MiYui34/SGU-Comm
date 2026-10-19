type Point = { x: number; y: number };
type Size = { width: number; height: number };

const MARGIN = 8;

// Native-menu placement: open at the pointer, flip to the other side when it
// does not fit, and never leave the window. up: open above the point (a
// button at the bottom of the window).
export const fitMenu = (at: Point, menu: Size, view: Size, up = false): Point => {
  const place = (pos: number, size: number, max: number) => {
    const flipped = pos + size > max - MARGIN ? pos - size : pos;
    return Math.max(MARGIN, Math.min(flipped, max - size - MARGIN));
  };
  const y = up && at.y - menu.height >= MARGIN ? at.y - menu.height : place(at.y, menu.height, view.height);
  return { x: place(at.x, menu.width, view.width), y };
};

// Arrow keys in a menu: the next item to focus (index -1: none yet), or
// null for other keys.
export const menuStep = (key: string, index: number, count: number): number | null => {
  if (count === 0) return null;
  switch (key) {
    case "ArrowDown":
      return (index + 1) % count;
    case "ArrowUp":
      return (index <= 0 ? count : index) - 1;
    case "Home":
      return 0;
    case "End":
      return count - 1;
    default:
      return null;
  }
};

// Where a menu opens: at the pointer, or below the element when opened with
// the keyboard (detail 0: no pointer position).
export const menuAnchor = (
  event: { detail: number; clientX: number; clientY: number },
  rect: { left: number; bottom: number },
): Point => (event.detail !== 0 ? { x: event.clientX, y: event.clientY } : { x: rect.left + 24, y: rect.bottom });
