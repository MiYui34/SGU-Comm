// Links to a stored message ("Copy link"): <server>/channels/<chat>/<id>.
// The server answers any such path with the app, which opens the chat and
// jumps to the message.
export const messageLink = (origin: string, chat: number, message: number) => `${origin}/channels/${chat}/${message}`;

// null: not a message link to this server (origin).
export const parseMessageLink = (url: string, origin: string): { chat: number; message: number } | null => {
  let u: URL;
  try {
    u = new URL(url, origin);
  } catch {
    return null;
  }
  const m = /^\/channels\/(\d+)\/(\d+)$/.exec(u.pathname);
  return u.origin === new URL(origin).origin && m ? { chat: Number(m[1]), message: Number(m[2]) } : null;
};
