<script lang="ts">
  import {
    joinChannel,
    messagesIn,
    privateMessagesWith,
    registerSelf,
    setSelfState,
    usersSorted,
  } from "@kallisti/protocol";
  import { tick, untrack, type ComponentProps } from "svelte";
  import type { AccountActions } from "../lib/accounts";
  import type { Connection } from "../lib/connection.svelte";
  import { isDesktop, registerGlobalPtt } from "../lib/desktop";
  import { nextPanel, swipeDirection, type Panel } from "../lib/mobile";
  import { pttAction, pttBlocksDefault } from "../lib/settings";
  import { unreadCounts } from "../lib/chat";
  import { htmlToText } from "../lib/htmlText";
  import { toChatMessage } from "../lib/textChannels";
  import { createTextView } from "../lib/textView.svelte";
  import { settings, userAudio } from "../lib/settings.svelte";
  import { gainOf, userAudioFor } from "../lib/userAudio";
  import type { SelfState } from "../lib/voice";
  import ChannelSidebar from "./ChannelSidebar.svelte";
  import ChatPane from "./ChatPane.svelte";
  import MemberList from "./MemberList.svelte";
  import ServerRail from "./ServerRail.svelte";
  import SettingsDialog from "./SettingsDialog.svelte";
  import Toast from "./Toast.svelte";
  import UserMenu from "./UserMenu.svelte";
  import UserPanel from "./UserPanel.svelte";

  let { connection, account }: { connection: Connection; account?: AccountActions } = $props();

  const server = $derived(connection.state);
  const me = $derived(server.session === null ? undefined : server.users[server.session]);
  const channelId = $derived(me?.channelId ?? 0);
  const channelName = $derived(server.channels[channelId]?.name ?? "");
  const serverName = $derived(server.channels[0]?.name ?? "Server");
  const users = $derived(usersSorted(server));

  const capture = $derived(connection.capture);
  // The server does not echo our own voice: add ourselves while sending.
  const speaking = $derived.by(() => {
    const others = connection.playback?.speaking ?? [];
    return new Set(capture?.transmitting && me ? [...others, me.session] : others);
  });

  $effect(() => connection.playback?.setDeaf(me?.selfDeaf ?? false));
  $effect(() => capture?.setMuted(me?.selfMute ?? false));

  let showSettings = $state(false);
  let membersOpen = $state(true);
  const prefs = $derived(settings.value);
  $effect(() => capture?.setMode(prefs.mode));
  // A rebound key never sends the keyup of the old one.
  $effect(() => {
    void prefs.pttKey;
    capture?.setPttDown(false);
  });
  $effect(() => capture?.setThreshold(prefs.vadThresholdDb));
  $effect(() => connection.playback?.setOutputDevice(prefs.outputDeviceId));
  // Opens the microphone, and reopens it when the device setting changes.
  // Its own derived: `prefs` is a new object on every settings change, and
  // only a device change may reopen the microphone.
  const inputDeviceId = $derived(prefs.inputDeviceId);
  $effect(() => {
    const [target, deviceId] = [capture, inputDeviceId];
    untrack(() => void target?.start(deviceId));
  });

  // Desktop app: the push-to-talk key works in the background too.
  // Own deriveds: `prefs` is a new object on every settings change, and
  // only the mode or the key may re-register the shortcut.
  const pttOn = $derived(prefs.mode === "ptt");
  const pttKey = $derived(prefs.pttKey);
  $effect(() => {
    if (!isDesktop() || !pttOn || !capture) return;
    const [key, target] = [pttKey, capture];
    let stop: (() => void) | null = null;
    let cancelled = false;
    registerGlobalPtt(key, (down) => target.setPttDown(down))
      .then((s) => (cancelled ? s() : (stop = s)))
      .catch((err) => console.warn("global push-to-talk", err));
    return () => {
      cancelled = true;
      stop?.();
    };
  });

  const isEditable = (t: EventTarget | null) =>
    t instanceof HTMLElement &&
    (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
  const onKey = (e: KeyboardEvent) => {
    if (e.type === "keydown" && e.key === "Escape" && phone && panel !== "chat" && !menu && !showSettings) {
      panel = "chat";
      return;
    }
    if (prefs.mode !== "ptt") return;
    const ev = { code: e.code, repeat: e.repeat, editable: isEditable(e.target) };
    if (pttBlocksDefault(ev, prefs.pttKey)) e.preventDefault();
    const action = pttAction(e.type as "keydown" | "keyup", ev, prefs.pttKey);
    if (action) capture?.setPttDown(action === "down");
  };

  // Phones: one panel at a time (see lib/mobile), switched by buttons or
  // swipes. Choosing a conversation goes back to the chat.
  const phoneQuery = matchMedia("(max-width: 700px)");
  let phone = $state(phoneQuery.matches);
  $effect(() => {
    const update = () => (phone = phoneQuery.matches);
    phoneQuery.addEventListener("change", update);
    return () => phoneQuery.removeEventListener("change", update);
  });
  let panel = $state<Panel>("chat");
  // Swipes do not count in dialogs, text fields, the hold button, or with
  // two fingers (pinch).
  const NO_SWIPE = "dialog, input, textarea, .hold, [role=dialog]";
  let touch: { x: number; y: number; t: number } | null = null;
  const touchStart = (e: TouchEvent) => {
    const t = e.touches[0];
    const blocked = e.touches.length > 1 || (e.target instanceof Element && e.target.closest(NO_SWIPE));
    touch = phone && t && !blocked ? { x: t.clientX, y: t.clientY, t: e.timeStamp } : null;
  };
  // An opened drawer gets the focus; closing it returns focus to the header
  // button that belongs to it (not the active element: Safari does not
  // focus buttons on click).
  let leftEl: HTMLElement | undefined = $state();
  let lastPanel: Panel = "chat";
  $effect(() => {
    if (!phone) return;
    const p = panel;
    untrack(async () => {
      const closed = lastPanel;
      lastPanel = p;
      await tick();
      if (p === "chat") {
        const button = closed === "channels" ? "button[aria-label=Channels]" : ".members-toggle";
        if (closed !== "chat") document.querySelector<HTMLElement>(button)?.focus();
        return;
      }
      const drawer = p === "channels" ? leftEl : document.querySelector<HTMLElement>(".layout > aside");
      drawer?.querySelector<HTMLElement>("button, [href], input")?.focus();
    });
  });
  const touchEnd = (e: TouchEvent) => {
    const t = e.changedTouches[0];
    if (!touch || !t) return;
    panel = nextPanel(panel, swipeDirection(touch, { x: t.clientX, y: t.clientY, t: e.timeStamp }));
    touch = null;
  };

  const paneCommon = $derived({
    menuOpen: panel === "channels",
    talking: capture?.transmitting ?? false,
    membersOpen: phone ? panel === "members" : membersOpen,
    onToggleMembers: () => {
      if (phone) panel = panel === "members" ? "chat" : "members";
      else membersOpen = !membersOpen;
    },
    onMenu: phone ? () => (panel = "channels") : undefined,
    // Phones have no PTT key: a hold button instead.
    onHoldTalk: phone && prefs.mode === "ptt" ? (down: boolean) => capture?.setPttDown(down) : undefined,
  });
  const pane = $derived.by(() => {
    if (view.kind === "text")
      return {
        title: textChannel?.name ?? "",
        kind: "text" as const,
        messages: textChat,
        welcomeText: "",
        more: text.more[view.id] ?? false,
        loadingOlder: textView.loadingOlder,
        onLoadOlder: textView.loadOlder,
        error: uploadError ?? text.lastError,
        onUpload: upload,
        uploading,
        onSend: send,
      } satisfies Partial<ComponentProps<typeof ChatPane>>;
    if (dm === null)
      return {
        title: channelName,
        topic: htmlToText(server.channels[channelId]?.description ?? ""),
        kind: "channel" as const,
        messages: messagesIn(server, channelId),
        welcomeText: channelId === 0 ? server.welcomeText : "",
        onSend: send,
      } satisfies Partial<ComponentProps<typeof ChatPane>>;
    return {
      title: dm.name,
      kind: "dm" as const,
      messages: privateMessagesWith(server, dm.name),
      welcomeText: "",
      onSend: dmPartner ? send : undefined,
      onClose: () => show({ kind: "voice" }),
    } satisfies Partial<ComponentProps<typeof ChatPane>>;
  });

  // An account that is not registered on Murmur yet registers itself once:
  // then the name is protected and shows as verified.
  let registerSent = false;
  $effect(() => {
    if (connection.account && me && me.userId === null && !registerSent) {
      registerSent = true;
      connection.send(registerSelf(me.session));
    }
  });

  const updateSelf = (s: SelfState) => {
    if (me) connection.send(setSelfState(me.session, s));
  };
  // Per-user volume and local mute, by user name.
  const audioOf = (name: string) => userAudioFor(userAudio.value, name);
  $effect(() => {
    for (const u of users) connection.playback?.setUserGain(u.session, gainOf(audioOf(u.name)));
  });
  const localMuted = $derived(new Set(users.filter((u) => audioOf(u.name).muted).map((u) => u.session)));

  // What the main pane shows. A direct message keeps the partner's name, so
  // it still shows after they left.
  type View =
    | { kind: "voice" }
    | { kind: "text"; id: number }
    | { kind: "dm"; session: number; name: string };
  let view = $state<View>({ kind: "voice" });
  let navigated = false;
  const show = (next: View) => {
    navigated = true;
    view = next;
    panel = "chat";
  };
  const dm = $derived(view.kind === "dm" ? view : null);

  // Text channels (Kallisti server only). Like Discord, open the first one
  // when they arrive, unless the user already picked something.
  const text = $derived(connection.text);
  const textId = $derived(view.kind === "text" ? view.id : null);
  const textChannel = $derived(text.channels.find((c) => c.id === textId));
  $effect(() => {
    const first = text.channels[0];
    if (first && !navigated && view.kind === "voice") view = { kind: "text", id: first.id };
  });
  // The open channel was removed: back to the voice view.
  $effect(() => {
    if (view.kind === "text" && text.available && !textChannel) view = { kind: "voice" };
  });
  const textView = createTextView(() => connection, () => textId);
  const textChat = $derived(
    textId === null ? [] : (text.messages[textId] ?? []).map((m) => toChatMessage(m, connection.httpBase)),
  );
  // One upload at a time. Errors show where server errors show.
  let uploading = $state(false);
  let uploadError = $state<string | null>(null);
  const upload = async (file: File, caption: string) => {
    if (view.kind !== "text" || uploading) return;
    const channel = view.id;
    uploading = true;
    uploadError = null;
    try {
      connection.sendToTextChannel(channel, caption, await connection.uploadFile(file));
    } catch (err) {
      uploadError = err instanceof Error ? err.message : String(err);
    } finally {
      uploading = false;
    }
  };

  // Open user menu.
  let menu = $state<{ session: number; x: number; y: number } | null>(null);
  const menuUser = $derived(menu && server.users[menu.session]);
  const openMenu = (session: number, event: MouseEvent) => {
    event.preventDefault();
    if (event.detail !== 0) {
      menu = { session, x: event.clientX, y: event.clientY };
      return;
    }
    // Opened with the keyboard: no pointer position, place it at the element.
    const r = (event.target as Element).getBoundingClientRect();
    menu = { session, x: r.left + 24, y: r.bottom };
  };
  // The user left: close their menu (Murmur reuses session numbers).
  $effect(() => {
    if (menu && !menuUser) menu = null;
  });

  // Unread direct messages per sender name, marked read while open.
  let readCounts = $state<Record<string, number>>({});
  const unread = $derived(unreadCounts(server.privateCounts, readCounts));
  $effect(() => {
    if (dm === null) return;
    const [name, count] = [dm.name, server.privateCounts[dm.name] ?? 0];
    untrack(() => {
      if (readCounts[name] !== count) readCounts = { ...readCounts, [name]: count };
    });
  });

  // Same name too: Murmur reuses the session numbers of users who left.
  const dmPartner = $derived.by(() => {
    const user = dm && server.users[dm.session];
    return user && user.name === dm?.name ? user : undefined;
  });
  // A new message or another view clears an old upload error.
  $effect(() => {
    void view;
    uploadError = null;
  });
  const send = (message: string) => {
    uploadError = null;
    if (view.kind === "text") connection.sendToTextChannel(view.id, message);
    else connection.sendText(dm === null ? { channelId } : { session: dm.session }, message);
  };
  const join = (id: number) => {
    show({ kind: "voice" });
    if (me && id !== channelId) connection.send(joinChannel(me.session, id));
  };
</script>

{#if server.notice}
  {#key server.notice.at}<Toast text={server.notice.text} />{/key}
{/if}

<svelte:window onkeydown={onKey} onkeyup={onKey} onblur={() => capture?.setPttDown(false)} />
<svelte:document ontouchstart={touchStart} ontouchend={touchEnd} />

{#if menu && menuUser}
  <UserMenu
    user={menuUser}
    isMe={menuUser.session === server.session}
    audio={audioOf(menuUser.name)}
    x={menu.x}
    y={menu.y}
    onAudio={(patch) => userAudio.update(menuUser.name, patch)}
    onMessage={() => show({ kind: "dm", session: menuUser.session, name: menuUser.name })}
    onSettings={() => (showSettings = true)}
    onClose={() => (menu = null)}
  />
{/if}

{#if showSettings}
  <SettingsDialog
    settings={prefs}
    level={capture?.level ?? -100}
    deviceLabel={capture?.deviceLabel ?? ""}
    fellBack={capture?.fellBack ?? false}
    onChange={settings.update}
    {account}
    onClose={() => (showSettings = false)}
  />
{/if}

<div
  class="layout"
  class:members={membersOpen}
  data-panel={phone ? panel : undefined}
>
  <div class="left" bind:this={leftEl} inert={phone && panel !== "channels"}>
  <ServerRail {serverName} />
  <div class="sidebar">
    <header>{serverName}</header>
    <ChannelSidebar
      {server}
      {speaking}
      {localMuted}
      {unread}
      textChannels={text.channels}
      currentText={textId}
      unreadText={textView.unread}
      onOpenText={(id) => show({ kind: "text", id })}
      currentChannel={view.kind === "voice" ? channelId : null}
      onJoin={join}
      onUserMenu={openMenu}
    />
    {#if me}
      <UserPanel
        {me}
        micError={capture?.micError ?? null}
        onSelfState={updateSelf}
        onSettings={() => (showSettings = true)}
      />
    {/if}
  </div>
  </div>
  {#if phone && panel !== "chat"}
    <button class="backdrop" aria-label="Close panel" onclick={() => (panel = "chat")}></button>
  {/if}
  <div class="main" inert={phone && panel !== "chat"}>
    <ChatPane {...paneCommon} {...pane} />
  </div>
  {#if membersOpen || phone}
    <MemberList
      inert={phone && panel !== "members"}
      {channelName}
      {unread}
      {speaking}
      onUserMenu={openMenu}
      here={users.filter((u) => u.channelId === channelId)}
      elsewhere={users.filter((u) => u.channelId !== channelId)}
    />
  {/if}
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: 72px 240px 1fr;
    height: 100%;
  }
  .layout.members {
    grid-template-columns: 72px 240px 1fr 240px;
  }
  /* Desktop: the wrapper does not exist for the grid. Phone: left drawer. */
  .left,
  .main {
    display: contents;
  }
  .backdrop {
    display: none;
  }
  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-sidebar);
  }
  header {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    height: 48px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border);
    font-size: 16px;
    font-weight: 600;
  }
  @media (max-width: 900px) and (min-width: 701px) {
    .layout,
    .layout.members {
      grid-template-columns: 72px 240px 1fr;
    }
    .layout > :global(aside) {
      display: none;
    }
  }
  /* Phones: the chat fills the screen, side panels slide over it. */
  @media (max-width: 700px) {
    .layout,
    .layout.members {
      position: relative;
      display: block;
      overflow: hidden;
    }
    .main {
      display: block;
      height: 100%;
    }
    .main > :global(main) {
      height: 100%;
    }
    .left,
    .layout > :global(aside) {
      position: absolute;
      top: 0;
      bottom: 0;
      z-index: 20;
      display: flex;
      transition: transform 0.2s ease;
    }
    .left {
      left: 0;
      width: min(312px, 88vw);
      transform: translateX(-100%);
    }
    .left > .sidebar {
      flex: 1;
      min-width: 0;
    }
    .layout > :global(aside) {
      right: 0;
      flex-direction: column;
      width: min(280px, 85vw);
      transform: translateX(100%);
    }
    [data-panel="channels"] .left,
    [data-panel="members"] > :global(aside) {
      transform: none;
    }
    .backdrop {
      position: absolute;
      inset: 0;
      z-index: 10;
      display: block;
      background: rgb(0 0 0 / 0.5);
    }
  }
</style>
