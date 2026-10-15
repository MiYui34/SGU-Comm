import { may, Perm, type ServerState, type User } from "@kallisti/protocol";
import type { TextState } from "./textChannels";

// Registered user ids by role (built-in server). SuperUser (id 0) is an
// admin too.
export type Roles = { admins: number[]; moderators: number[] };
export type Role = "admin" | "moderator";

// A registered user's role (null: none, or a guest).
export const roleOf = (roles: Roles, userId: number | null): Role | null =>
  userId === null
    ? null
    : userId === 0 || roles.admins.includes(userId)
      ? "admin"
      : roles.moderators.includes(userId)
        ? "moderator"
        : null;

// What an admin may do about a user's role (built-in server): make a
// registered user a moderator, or not. Admins stay admins.
export const moderatorAction = (server: ServerState, text: TextState, user: User): "make" | "remove" | null => {
  if (!text.voiceChats || !may(server, 0, Perm.Write) || user.userId === null) return null;
  const role = roleOf(text.roles, user.userId);
  return role === "admin" ? null : role === "moderator" ? "remove" : "make";
};
