type Point = { x: number; y: number };
type Size = { width: number; height: number };

const MARGIN = 8;

// Native-menu placement: open at the pointer, flip to the other side when it
// does not fit, and never leave the window.
export const fitMenu = (at: Point, menu: Size, view: Size): Point => {
  const place = (pos: number, size: number, max: number) => {
    const flipped = pos + size > max - MARGIN ? pos - size : pos;
    return Math.max(MARGIN, Math.min(flipped, max - size - MARGIN));
  };
  return { x: place(at.x, menu.width, view.width), y: place(at.y, menu.height, view.height) };
};

// Where a menu opens: at the pointer, or below the element when opened with
// the keyboard (detail 0: no pointer position).
export const menuAnchor = (
  event: { detail: number; clientX: number; clientY: number },
  rect: { left: number; bottom: number },
): Point => (event.detail !== 0 ? { x: event.clientX, y: event.clientY } : { x: rect.left + 24, y: rect.bottom });
