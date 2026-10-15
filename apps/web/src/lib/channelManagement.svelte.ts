import { channelSiblings, createChannel, editChannel, may, Perm, queryPermissions, removeChannel } from "@kallisti/protocol";
import type { ChannelForm, ChannelKind } from "./channelForm";
import { canManageText, menuRights } from "./channelManagement";
import { moveInList, moveVoiceChannel } from "./channelOrder";
import type { Connection } from "./connection.svelte";
import { menuAnchor } from "./menu";

export type ChannelValues = { kind: ChannelKind; name: string; description: string; temporary: boolean };

// The channel menu and dialog: what they show and what they send.
export const createChannelManagement = (current: () => Connection) => {
  let opened = $state<{ kind: ChannelKind; id: number; x: number; y: number } | null>(null);
  let form = $state<{ form: ChannelForm; id: number } | null>(null);
  const connection = $derived(current());
  const server = $derived(connection.state);
  const text = $derived(connection.text);
  const canMakeVoice = $derived(may(server, 0, Perm.MakeChannel));
  const canCreate = $derived({ text: canManageText(server, text), voice: canMakeVoice || may(server, 0, Perm.MakeTempChannel) });
  // A menu whose channel went away is closed.
  const menu = $derived.by(() => {
    const rights = opened && menuRights(server, text, opened.kind, opened.id);
    return opened && rights ? { ...opened, ...rights, name: nameOf(opened.kind, opened.id) } : null;
  });

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
    open: (kind: ChannelKind, id: number, event: MouseEvent) => {
      event.preventDefault();
      if (kind === "text" && !canManageText(server, text)) return;
      // Ask what we may do here and with the siblings (for moves). Root
      // has no siblings.
      if (kind === "voice") {
        const ids = channelSiblings(server, id).map((c) => c.id);
        for (const c of ids.length ? ids : [id]) connection.send(queryPermissions(c));
      }
      opened = { kind, id, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
    },
    closeMenu: () => (opened = null),
    startCreate: (kind: ChannelKind) =>
      (form = { form: { mode: "create", kind, temporaryOnly: kind === "voice" && !canMakeVoice }, id: 0 }),
    startEdit: (kind: ChannelKind, id: number) =>
      (form = {
        form: { mode: "edit", kind, name: nameOf(kind, id), description: kind === "voice" ? (server.channels[id]?.description ?? "") : "" },
        id,
      }),
    startDelete: (kind: ChannelKind, id: number) => (form = { form: { mode: "delete", kind, name: nameOf(kind, id) }, id }),
    closeForm: () => (form = null),
    move: (kind: ChannelKind, id: number, step: -1 | 1) => {
      if (kind === "text") connection.manageTextChannels({ order: moveInList(text.channels.map((c) => c.id), id, step) });
      else for (const c of moveVoiceChannel(channelSiblings(server, id), id, step)) connection.send(editChannel(c.id, { position: c.position }));
    },
    submit: (v: ChannelValues) => {
      if (!form) return;
      const { form: f, id } = form;
      if (f.mode === "create") {
        if (v.kind === "text") connection.manageTextChannels({ create: v.name });
        else connection.send(createChannel(0, v.name, { temporary: v.temporary }));
      } else if (f.mode === "edit") {
        if (v.kind === "text") connection.manageTextChannels({ rename: id, name: v.name });
        else {
          const changes = {
            ...(v.name !== f.name ? { name: v.name } : {}),
            ...(v.description !== f.description ? { description: v.description } : {}),
          };
          if (Object.keys(changes).length) connection.send(editChannel(id, changes));
        }
      } else if (v.kind === "text") connection.manageTextChannels({ remove: id });
      else connection.send(removeChannel(id));
    },
  };
};
