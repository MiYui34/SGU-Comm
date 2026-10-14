const COLORS = ["#e2745b", "#d9a33b", "#5aa469", "#3ba99c", "#4f8fd6", "#8a6fd1", "#c763a6", "#7c8794"];

export const initials = (name: string) => [...name.trim()][0]?.toUpperCase() ?? "?";

// Small string hash (FNV-1a) so a name always gets the same color.
export const avatarColor = (name: string) => {
  let hash = 0x811c9dc5;
  for (const ch of name) hash = Math.imul(hash ^ ch.codePointAt(0)!, 0x01000193);
  return COLORS[(hash >>> 0) % COLORS.length]!;
};
