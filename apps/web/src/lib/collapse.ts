// Long messages show collapsed (see ChatPane): more than MAX_LINES lines
// of the written text. Code blocks count with their lines (and fences).
export const MAX_LINES = 20;
// About the characters on one line of a wide chat: longer lines wrap.
export const CHARS_PER_LINE = 120;
// Content we cannot count (images in Mumble HTML): its measured height.
export const MAX_HEIGHT_PX = 400;

export const textLines = (text: string, perLine = CHARS_PER_LINE) =>
  text.split("\n").reduce((n, line) => n + Math.max(1, Math.ceil(line.length / perLine)), 0);

export const collapses = (text: string, max = MAX_LINES, perLine = CHARS_PER_LINE) => textLines(text, perLine) > max;
