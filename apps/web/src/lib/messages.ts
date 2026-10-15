import type { ChatMessage, User } from "@kallisti/protocol";

// Our own stored messages, as the server sees it: registered users own
// verified messages with their name; guests (free names) only what they
// wrote since they joined. The server checks again.
export const ownsMessage = (m: ChatMessage, me: User | undefined, joinedAt: number): boolean =>
  !!me &&
  m.actorName?.toLowerCase() === me.name.toLowerCase() &&
  ("verified" in m && m.verified === true) === (me.userId !== null) &&
  (me.userId !== null || m.at >= joinedAt);
