import { channelSiblings, may, Perm, type ServerState } from "@kallisti/protocol";
import type { ChannelKind } from "./channelForm";
import { mirrorIds } from "./mirrors";
import { accessChannel } from "./privateChannels";
import { channelModIn, myUserId } from "./roles";
import type { TextState } from "./textChannels";

export type MenuRights = { canEdit: boolean; canDelete: boolean; canMoveUp: boolean; canMoveDown: boolean };

// Text channels: admins and moderators (MakeChannel in root), with the
// built-in server.
export const canManageText = (server: ServerState, text: TextState): boolean =>
  text.voiceChats && may(server, 0, Perm.MakeChannel);

// We moderate a Mumble channel (a voice channel or a text channel's
// mirror; null: none): staff everywhere, its moderators there.
export const canModerate = (server: ServerState, text: TextState, channel: number | null): boolean =>
  canManageText(server, text) || (text.voiceChats && channelModIn(server, text.roles, channel, myUserId(server)));

// The members of a private channel: admins, moderators and its moderators.
export const canManageMembers = (server: ServerState, text: TextState, kind: ChannelKind, id: number): boolean => {
  const access = accessChannel(kind, id, text.channels);
  return access !== null && access in text.private && canModerate(server, text, access);
};

// A channel's moderators (staff appoint them): the Mumble channel that
// holds them, or null. Not root: its moderators are the server's.
export const moderatorsChannel = (server: ServerState, text: TextState, kind: ChannelKind, id: number): number | null => {
  const access = accessChannel(kind, id, text.channels);
  return canManageText(server, text) && access !== null && access !== 0 && server.channels[access] ? access : null;
};

// The listen-only switch on edit (built-in server, not root). It needs
// Write from the ACLs: not for channel moderators in their channels (their
// Write there only manages subchannels).
export const canListenOnly = (server: ServerState, text: TextState, id: number): boolean =>
  text.voiceChats && id !== 0 && (canManageText(server, text) || !channelModIn(server, text.roles, id, myUserId(server)));

// "Create subchannel" in a voice channel's menu: MakeChannel there (staff,
// and channel moderators in their channels and below). Not root (the "+"
// makes those), temporary channels (they have none) nor text channel
// mirrors.
export const canCreateSubchannel = (server: ServerState, text: TextState, id: number): boolean =>
  id !== 0 && server.channels[id]?.temporary === false && !mirrorIds(server, text.channels).has(id) && may(server, id, Perm.MakeChannel);

// A voice channel and its siblings, without text channel mirrors (they
// keep their place; nobody but SuperUser may edit them).
export const voiceSiblings = (server: ServerState, text: TextState, id: number) => {
  const hidden = mirrorIds(server, text.channels);
  return channelSiblings(server, id).filter((c) => !hidden.has(c.id));
};

// What a channel's menu offers (null: no menu; text channels always have
// one, to mute them). A text channel's moderators edit its topic and slow
// mode. Voice channels follow
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
    const own = canModerate(server, text, accessChannel(kind, id, text.channels));
    return rights(own, manage, text.channels.map((c) => c.id), manage);
  }
  if (!server.channels[id]) return null;
  const siblings = voiceSiblings(server, text, id);
  const canEdit = may(server, id, Perm.Write);
  return rights(canEdit, canEdit && id !== 0, siblings.map((c) => c.id), siblings.every((c) => may(server, c.id, Perm.Write)));
};
