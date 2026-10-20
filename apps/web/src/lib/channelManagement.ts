import { channelSiblings, may, Perm, type ServerState } from "@kallisti/protocol";
import { textGroups } from "./categories";
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

// A category's Mumble channel (null: none, or no such category).
const categoryMumble = (text: TextState, id: number | undefined): number | null =>
  text.categories.find((c) => c.id === id)?.mumble ?? null;

// We manage a category's text channels: staff all, its moderators theirs
// (undefined: none).
export const managesCategory = (server: ServerState, text: TextState, category: number | undefined): boolean =>
  text.categories.some((c) => c.id === category) && canModerate(server, text, categoryMumble(text, category));

// We may create a text channel in a category (undefined: none).
export const canCreateText = (server: ServerState, text: TextState, category?: number): boolean =>
  category === undefined ? canManageText(server, text) : managesCategory(server, text, category);

// The members of a private channel: admins, moderators and its moderators.
export const canManageMembers = (server: ServerState, text: TextState, kind: ChannelKind, id: number): boolean => {
  const access = accessChannel(kind, id, text.channels);
  return access !== null && access in text.private && canModerate(server, text, access);
};

// A channel's moderators (staff appoint them): the Mumble channel that
// holds them, or null. Not root: its moderators are the server's.
export const moderatorsChannel = (server: ServerState, text: TextState, kind: ChannelKind, id: number): number | null => {
  const access = kind === "category" ? categoryMumble(text, id) : accessChannel(kind, id, text.channels);
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
  id !== 0 && server.channels[id]?.temporary === false && !mirrorIds(server, text.channels, text.categories).has(id) && may(server, id, Perm.MakeChannel);

// A voice channel and its siblings, without text channel mirrors (they
// keep their place; nobody but SuperUser may edit them).
export const voiceSiblings = (server: ServerState, text: TextState, id: number) => {
  const hidden = mirrorIds(server, text.channels, text.categories);
  return channelSiblings(server, id).filter((c) => !hidden.has(c.id));
};

// What a channel's menu offers (null: no menu; text channels always have
// one, to mute them). A text channel's moderators edit its topic and slow
// mode; a category's moderators also delete and move its channels (within
// it, as staff). Categories are for staff. Voice channels follow
// Mumble's permissions, known after a queryPermissions. A move renumbers
// all siblings, so it needs Write on each of them.
export const menuRights = (server: ServerState, text: TextState, kind: ChannelKind, id: number): MenuRights | null => {
  const rights = (canEdit: boolean, canDelete: boolean, order: number[], canMove: boolean) => {
    const at = order.indexOf(id);
    return { canEdit, canDelete, canMoveUp: canMove && at > 0, canMoveDown: canMove && at >= 0 && at < order.length - 1 };
  };
  if (kind === "category") {
    const manage = canManageText(server, text);
    return manage && text.categories.some((c) => c.id === id) ? rights(true, true, text.categories.map((c) => c.id), true) : null;
  }
  if (kind === "text") {
    const group = textGroups(text.channels, text.categories).find((g) => g.channels.some((c) => c.id === id));
    if (!group) return null;
    const manage = canManageText(server, text) || managesCategory(server, text, group.category?.id);
    const own = canModerate(server, text, accessChannel(kind, id, text.channels));
    return rights(own, manage && own, group.channels.map((c) => c.id), manage);
  }
  if (!server.channels[id]) return null;
  const siblings = voiceSiblings(server, text, id);
  const canEdit = may(server, id, Perm.Write);
  return rights(canEdit, canEdit && id !== 0, siblings.map((c) => c.id), siblings.every((c) => may(server, c.id, Perm.Write)));
};
