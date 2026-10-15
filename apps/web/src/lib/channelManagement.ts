import { channelSiblings, may, Perm, type ServerState } from "@kallisti/protocol";
import type { ChannelKind } from "./channelForm";
import type { TextState } from "./textChannels";

export type MenuRights = { canEdit: boolean; canDelete: boolean; canMoveUp: boolean; canMoveDown: boolean };

// Text channels: admins (Write in root), with the built-in server.
export const canManageText = (server: ServerState, text: TextState): boolean =>
  text.voiceChats && may(server, 0, Perm.Write);

// What a channel's menu offers (null: no menu). Voice channels follow
// Mumble's permissions, known after a queryPermissions. A move renumbers
// all siblings, so it needs Write on each of them.
export const menuRights = (server: ServerState, text: TextState, kind: ChannelKind, id: number): MenuRights | null => {
  const rights = (canEdit: boolean, canDelete: boolean, order: number[], canMove: boolean) => {
    const at = order.indexOf(id);
    return { canEdit, canDelete, canMoveUp: canMove && at > 0, canMoveDown: canMove && at >= 0 && at < order.length - 1 };
  };
  if (kind === "text") {
    if (!canManageText(server, text) || !text.channels.some((c) => c.id === id)) return null;
    return rights(true, true, text.channels.map((c) => c.id), true);
  }
  if (!server.channels[id]) return null;
  const siblings = channelSiblings(server, id);
  const canEdit = may(server, id, Perm.Write);
  return rights(canEdit, canEdit && id !== 0, siblings.map((c) => c.id), siblings.every((c) => may(server, c.id, Perm.Write)));
};
