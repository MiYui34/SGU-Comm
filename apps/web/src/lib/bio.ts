// The bio ("About me") is the Mumble user comment. Short, so it fits the
// user menu (the server allows much longer comments).
export const BIO_MAX = 190;

// The bio to send: trimmed (the server trims too) and cut to the limit;
// null when it did not change (a longer one from a Mumble app stays).
export const bioToSave = (draft: string, current: string): string | null => {
  const text = draft.trim();
  return text === current ? null : text.slice(0, BIO_MAX);
};
