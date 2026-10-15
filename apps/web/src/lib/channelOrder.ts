// Moving channels up and down in the sidebar.

export const moveInList = (ids: readonly number[], id: number, step: -1 | 1): number[] => {
  const out = [...ids];
  const i = out.indexOf(id);
  const j = i + step;
  if (i < 0 || j < 0 || j >= out.length) return out;
  [out[i], out[j]] = [out[j]!, out[i]!];
  return out;
};

// Voice channels: the siblings in their shown order, with the moved one one
// step further. Returns the position changes to send: siblings get 0, 1,
// 2, ... (Mumble sorts by position, then name, so equal positions must go).
export const moveVoiceChannel = (
  siblings: readonly { id: number; position: number }[],
  id: number,
  step: -1 | 1,
): { id: number; position: number }[] => {
  const order = moveInList(siblings.map((s) => s.id), id, step);
  if (order.every((sid, i) => sid === siblings[i]!.id)) return [];
  const position = new Map(siblings.map((s) => [s.id, s.position]));
  return order.map((sid, i) => ({ id: sid, position: i })).filter((c) => position.get(c.id) !== c.position);
};
