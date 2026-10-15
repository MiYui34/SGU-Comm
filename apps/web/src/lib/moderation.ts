import { may, Perm, type ServerState } from "@kallisti/protocol";

// What the server settings may show (the server checks again): the ban
// list with Ban in root, changing it with Write too (admins); registered
// users with Register.
export const moderationRights = (server: ServerState) => {
  const bans = may(server, 0, Perm.Ban);
  const users = may(server, 0, Perm.Register);
  return { any: bans || users, bans, editBans: bans && may(server, 0, Perm.Write), users };
};
