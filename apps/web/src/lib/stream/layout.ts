// The stream stage's layout: a grid of all watched streams, or one in
// focus (large) and the others in a strip.

export type StageView = "grid" | "focus";

// The number of columns that makes n tiles of the given aspect the largest
// in a width x height area (gap: between tiles). Ties: fewer columns.
export const gridColumns = (n: number, width: number, height: number, aspect = 16 / 9, gap = 8): number => {
  if (n <= 1 || width <= 0 || height <= 0) return 1;
  let best = 1;
  let bestWidth = -Infinity;
  for (let cols = 1; cols <= n; cols++) {
    const rows = Math.ceil(n / cols);
    const tile = Math.min((width - gap * (cols - 1)) / cols, ((height - gap * (rows - 1)) / rows) * aspect);
    if (tile > bestWidth) [best, bestWidth] = [cols, tile];
  }
  return best;
};

// The stream in focus after the watched ones changed from before to after:
// one just watched (it was clicked), else the same while it is there, else
// the first left.
export const nextFocus = (focused: number | null, before: number[], after: number[]): number | null =>
  after.find((s) => !before.includes(s)) ?? (focused !== null && after.includes(focused) ? focused : (after[0] ?? null));

// The streams in the strip: all but the focused one, in order.
export const stripOf = (sessions: number[], focused: number | null): number[] => sessions.filter((s) => s !== focused);

// A full screen button: leaves full screen when its element is the one in
// full screen, else puts it there (a tile of the full screen stage too: its
// own button then goes back to the stage).
export const fullScreenStep = <T>(current: T | null, target: T): "enter" | "exit" => (current === target ? "exit" : "enter");
