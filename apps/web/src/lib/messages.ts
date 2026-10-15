import type { ChatMessage, User } from "@kallisti/protocol";

// Our own stored messages, as the server sees it: registered users own
// verified messages with their user id (old messages without one: with
// their name); guests (free names) only what they wrote since they joined.
// The server checks again.
export const ownsMessage = (m: ChatMessage & { authorId?: number }, me: User | undefined, joinedAt: number): boolean => {
  if (!me) return false;
  const verified = "verified" in m && m.verified === true;
  if (verified && m.authorId !== undefined) return m.authorId === me.userId;
  return (
    m.actorName?.toLowerCase() === me.name.toLowerCase() &&
    verified === (me.userId !== null) &&
    (me.userId !== null || m.at >= joinedAt)
  );
};
