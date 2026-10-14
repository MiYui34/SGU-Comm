type ScrollState = { scrollHeight: number; scrollTop: number; clientHeight: number };

const AT_BOTTOM_PX = 80;

// Where the chat should scroll after its content changed: follow the bottom
// if the reader was there (or the conversation changed), else keep what the
// reader sees in place. `prepended`: content was added above (older history).
export const nextScrollTop = (
  before: ScrollState,
  newScrollHeight: number,
  switched: boolean,
  prepended = false,
): number => {
  const fromBottom = before.scrollHeight - before.scrollTop - before.clientHeight;
  if (switched || fromBottom <= AT_BOTTOM_PX) return newScrollHeight - before.clientHeight;
  return prepended ? before.scrollTop + (newScrollHeight - before.scrollHeight) : before.scrollTop;
};
