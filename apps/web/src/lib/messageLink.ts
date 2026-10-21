// Links to a stored message ("Copy link"): <server>/channels/<chat>/<id>,
// and to a chat: <server>/channels/<chat>. The server answers any such
// path with the app, which opens the chat and jumps to the message.
export const messageLink = (origin: string, chat: number, message: number) => `${origin}/channels/${chat}/${message}`;
export const channelLink = (origin: string, chat: number) => `${origin}/channels/${chat}`;

// null: not a message or chat link to this server (origin).
export const parseMessageLink = (url: string, origin: string): { chat: number; message?: number } | null => {
  let u: URL;
  try {
    u = new URL(url, origin);
  } catch {
    return null;
  }
  const m = /^\/channels\/(\d+)(?:\/(\d+))?$/.exec(u.pathname);
  if (u.origin !== new URL(origin).origin || !m) return null;
  return m[2] ? { chat: Number(m[1]), message: Number(m[2]) } : { chat: Number(m[1]) };
};
