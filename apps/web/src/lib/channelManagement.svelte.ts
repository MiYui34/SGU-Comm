import { createChannel, editChannel, may, Perm, queryPermissions, removeChannel } from "@kallisti/protocol";
import type { ChannelForm, ChannelKind } from "./channelForm";
import { canCreateSubchannel, canManageMembers, canManageText, canModerate, menuRights, moderatorsChannel, voiceSiblings } from "./channelManagement";
import { channelModActions } from "./roles";
import { listenOnlyAction, speakerCandidates, type ListenOnly } from "./listenOnly";
import { accessChannel, membersAction, privateAction, type Member } from "./privateChannels";
import { moveInList, moveVoiceChannel } from "./channelOrder";
import type { Connection } from "./connection.svelte";
import { menuAnchor } from "./menu";

export type ChannelValues = {
  kind: ChannelKind;
  name: string;
  description: string;
  temporary: boolean;
  listenOnly?: ListenOnly;
  private?: boolean;
  slowMode?: number;
};

// The channel menu and dialog: what they show and what they send.
export const createChannelManagement = (current: () => Connection) => {
  let opened = $state<{ kind: ChannelKind; id: number; x: number; y: number } | null>(null);
  let form = $state<{ form: ChannelForm; id: number } | null>(null);
  // The members dialog of a private channel, and the moderators dialog
  // (by the Mumble channel).
  let members = $state<{ kind: ChannelKind; id: number; access: number } | null>(null);
  let moderators = $state<{ kind: ChannelKind; id: number; access: number } | null>(null);
  const connection = $derived(current());
  const server = $derived(connection.state);
  const text = $derived(connection.text);
  const canMakeVoice = $derived(may(server, 0, Perm.MakeChannel));
  const canCreate = $derived({ text: canManageText(server, text), voice: canMakeVoice || may(server, 0, Perm.MakeTempChannel) });
  // A menu whose channel went away is closed.
  const menu = $derived.by(() => {
    const rights = opened && menuRights(server, text, opened.kind, opened.id);
    return opened && rights
      ? {
          ...opened,
          ...rights,
          canMembers: canManageMembers(server, text, opened.kind, opened.id),
          canCreateSub: opened.kind === "voice" && canCreateSubchannel(server, text, opened.id),
          canModerators: moderatorsChannel(server, text, opened.kind, opened.id) !== null,
          name: nameOf(opened.kind, opened.id),
        }
      : null;
  });
  // The moderators dialog: their names from the registered users we know.
  const moderatorsForm = $derived.by(() =>
    moderators && server.channels[moderators.access]
      ? {
          ...moderators,
          name: nameOf(moderators.kind, moderators.id),
          members: (text.roles.channelMods?.[moderators.access] ?? []).map((id) => ({ id, name: text.members[id] ?? `#${id}` })),
        }
      : null,
  );
  // A members dialog whose channel stopped being private is closed.
  const membersForm = $derived.by(() =>
    members && members.access in text.private
      ? { ...members, name: nameOf(members.kind, members.id), members: text.private[members.access] ?? [] }
      : null,
  );

  // The private switch on edit: admins and moderators, not root, and text
  // channels only with a mirror.
  const privateField = (kind: ChannelKind, id: number) => {
    const access = accessChannel(kind, id, text.channels);
    if (!canManageText(server, text) || access === null || access === 0) return {};
    return { private: { on: access in text.private, members: text.private[access] ?? [] } };
  };

  const nameOf = (kind: ChannelKind, id: number) =>
    kind === "text" ? (text.channels.find((c) => c.id === id)?.name ?? "") : (server.channels[id]?.name ?? "");

  return {
    get menu() {
      return menu;
    },
    get form() {
      return form;
    },
    get canCreate() {
      return canCreate;
    },
    get members() {
      return membersForm;
    },
    get moderators() {
      return moderatorsForm;
    },
    open: (kind: ChannelKind, id: number, event: MouseEvent) => {
      event.preventDefault();
      // Ask what we may do here and with the siblings (for moves). Root
      // has no siblings.
      if (kind === "voice") {
        const ids = voiceSiblings(server, text, id).map((c) => c.id);
        for (const c of ids.length ? ids : [id]) connection.send(queryPermissions(c));
      }
      opened = { kind, id, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
    },
    closeMenu: () => (opened = null),
    // parent: a voice subchannel there (its moderators may make private
    // ones too).
    startCreate: (kind: ChannelKind, parent?: number) =>
      (form = {
        form: parent
          ? { mode: "create", kind, temporaryOnly: false, canPrivate: canModerate(server, text, parent), parent }
          : { mode: "create", kind, temporaryOnly: kind === "voice" && !canMakeVoice, canPrivate: canManageText(server, text) },
        id: 0,
      }),
    startEdit: (kind: ChannelKind, id: number) =>
      (form = {
        form: {
          mode: "edit",
          kind,
          name: nameOf(kind, id),
          // Text channels: the topic (plain text).
          description: kind === "voice" ? (server.channels[id]?.description ?? "") : (text.channels.find((c) => c.id === id)?.topic ?? ""),
          ...(kind === "voice" && id !== 0 && text.voiceChats
            ? { listenOnly: { on: id in text.listenOnly, speakers: text.listenOnly[id] ?? [], users: speakerCandidates(server) } }
            : {}),
          ...privateField(kind, id),
          ...(kind === "text" && canModerate(server, text, accessChannel(kind, id, text.channels))
            ? { slowMode: text.channels.find((c) => c.id === id)?.slowMode ?? 0 }
            : {}),
          ...(kind === "text" && !canManageText(server, text) ? { topicOnly: true } : {}),
        },
        id,
      }),
    startMembers: (kind: ChannelKind, id: number) => {
      const access = accessChannel(kind, id, text.channels);
      if (access !== null) members = { kind, id, access };
    },
    closeMembers: () => (members = null),
    startModerators: (kind: ChannelKind, id: number) => {
      const access = moderatorsChannel(server, text, kind, id);
      if (access !== null) moderators = { kind, id, access };
    },
    closeModerators: () => (moderators = null),
    submitModerators: (after: Member[]) => {
      const f = moderatorsForm;
      if (!f) return;
      for (const a of channelModActions(f.access, f.members.map((m) => m.id), after.map((m) => m.id))) connection.manage(a);
    },
    submitMembers: (after: Member[]) => {
      const action = membersForm && membersAction(membersForm.access, membersForm.members, after);
      if (action) connection.manage(action);
    },
    startDelete: (kind: ChannelKind, id: number) => (form = { form: { mode: "delete", kind, name: nameOf(kind, id) }, id }),
    closeForm: () => (form = null),
    move: (kind: ChannelKind, id: number, step: -1 | 1) => {
      if (kind === "text") connection.manage({ order: moveInList(text.channels.map((c) => c.id), id, step) });
      else for (const c of moveVoiceChannel(voiceSiblings(server, text, id), id, step)) connection.send(editChannel(c.id, { position: c.position }));
    },
    submit: (v: ChannelValues) => {
      if (!form) return;
      const { form: f, id } = form;
      if (f.mode === "create") {
        if (v.kind === "text") connection.manage({ create: v.name, private: v.private });
        else if (v.private) connection.manage({ createPrivateVoice: v.name, parent: f.parent });
        else connection.send(createChannel(f.parent ?? 0, v.name, { temporary: v.temporary }));
      } else if (f.mode === "edit") {
        const access = accessChannel(v.kind, id, text.channels);
        const priv = f.private && access !== null && privateAction(access, f.private.on, v.private ?? false, f.private.members);
        if (priv) connection.manage(priv);
        if (v.kind === "text") {
          if (v.name !== f.name) connection.manage({ rename: id, name: v.name });
          if (v.description.trim() !== f.description) connection.manage({ topic: id, text: v.description.trim() });
          if (v.slowMode !== undefined && v.slowMode !== f.slowMode) connection.manage({ slowMode: id, seconds: v.slowMode });
        } else {
          const changes = {
            ...(v.name !== f.name ? { name: v.name } : {}),
            ...(v.description !== f.description ? { description: v.description } : {}),
          };
          if (Object.keys(changes).length) connection.send(editChannel(id, changes));
          const listenOnly = f.listenOnly && v.listenOnly && listenOnlyAction(id, f.listenOnly, v.listenOnly);
          if (listenOnly) connection.manage(listenOnly);
        }
      } else if (v.kind === "text") connection.manage({ remove: id });
      else connection.send(removeChannel(id));
    },
  };
};
