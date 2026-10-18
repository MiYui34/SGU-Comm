import { channelSiblings, may, Perm, type ServerState } from "@kallisti/protocol";
import type { ChannelKind } from "./channelForm";
import { mirrorIds } from "./mirrors";
import { accessChannel } from "./privateChannels";
import type { TextState } from "./textChannels";

export type MenuRights = { canEdit: boolean; canDelete: boolean; canMoveUp: boolean; canMoveDown: boolean };

// Text channels: admins and moderators (MakeChannel in root), with the
// built-in server.
export const canManageText = (server: ServerState, text: TextState): boolean =>
  text.voiceChats && may(server, 0, Perm.MakeChannel);

// The members of a private channel: admins and moderators.
export const canManageMembers = (server: ServerState, text: TextState, kind: ChannelKind, id: number): boolean => {
  const access = accessChannel(kind, id, text.channels);
  return canManageText(server, text) && access !== null && access in text.private;
};

// A voice channel and its siblings, without text channel mirrors (they
// keep their place; nobody but SuperUser may edit them).
export const voiceSiblings = (server: ServerState, text: TextState, id: number) => {
  const hidden = mirrorIds(server, text.channels);
  return channelSiblings(server, id).filter((c) => !hidden.has(c.id));
};

// What a channel's menu offers (null: no menu; text channels always have
// one, to mute them). Voice channels follow
// Mumble's permissions, known after a queryPermissions. A move renumbers
// all siblings, so it needs Write on each of them.
export const menuRights = (server: ServerState, text: TextState, kind: ChannelKind, id: number): MenuRights | null => {
  const rights = (canEdit: boolean, canDelete: boolean, order: number[], canMove: boolean) => {
    const at = order.indexOf(id);
    return { canEdit, canDelete, canMoveUp: canMove && at > 0, canMoveDown: canMove && at >= 0 && at < order.length - 1 };
  };
  if (kind === "text") {
    if (!text.channels.some((c) => c.id === id)) return null;
    const manage = canManageText(server, text);
    return rights(manage, manage, text.channels.map((c) => c.id), manage);
  }
  if (!server.channels[id]) return null;
  const siblings = voiceSiblings(server, text, id);
  const canEdit = may(server, id, Perm.Write);
  return rights(canEdit, canEdit && id !== 0, siblings.map((c) => c.id), siblings.every((c) => may(server, c.id, Perm.Write)));
};
