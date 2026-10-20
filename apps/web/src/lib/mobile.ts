// Phone layout: one panel at a time, switched by buttons or swipes, like
// Discord's mobile app.
export type Panel = "channels" | "chat" | "members";
type TouchPoint = { x: number; y: number; t: number };

// A quick, mostly horizontal swipe. Vertical movement is scrolling.
export const swipeDirection = (start: TouchPoint, end: TouchPoint): "left" | "right" | null => {
  const dx = end.x - start.x;
  const dy = end.y - start.y;
  if (Math.abs(dx) < 60 || Math.abs(dy) > Math.abs(dx) * 0.6 || end.t - start.t > 800) return null;
  return dx > 0 ? "right" : "left";
};

// Swiping right reveals what is to the left, like turning pages.
export const nextPanel = (panel: Panel, swipe: "left" | "right" | null): Panel => {
  if (swipe === "right") return panel === "chat" ? "channels" : panel === "members" ? "chat" : panel;
  if (swipe === "left") return panel === "chat" ? "members" : panel === "channels" ? "chat" : panel;
  return panel;
};

// What the right column (the right drawer on phones) shows: the members, or
// search results, the open chat's pins or a text channel's threads in
// their place.
export type Side = "members" | "search" | "pins" | "threads";

// The pins (or threads) button. shown: the column is on screen (not a
// closed drawer).
export const toggleSide = (side: Side, shown: boolean, which: Side): Side => (side === which && shown ? "members" : which);
