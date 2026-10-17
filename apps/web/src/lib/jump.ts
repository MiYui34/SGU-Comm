// Jumping to a stored message (a search result, a reply's original): page
// back until it is loaded, a few pages at most.
export const MAX_JUMP_PAGES = 10;

export type JumpState = { loaded: boolean; more: boolean; loading: boolean; pages: number };

export const jumpStep = ({ loaded, more, loading, pages }: JumpState): "jump" | "load" | "wait" | "give-up" => {
  if (loaded) return "jump";
  if (loading) return "wait";
  return more && pages < MAX_JUMP_PAGES ? "load" : "give-up";
};
