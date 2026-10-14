<script lang="ts">
  import {
    joinChannel,
    messagesIn,
    privateMessagesWith,
    setSelfState,
    usersSorted,
  } from "@mumble-discord/protocol";
  import { untrack } from "svelte";
  import type { Connection } from "../lib/connection.svelte";
  import { pttAction } from "../lib/settings";
  import { privateCountsFrom, unreadCounts } from "../lib/chat";
  import { settings, userAudio } from "../lib/settings.svelte";
  import { gainOf, userAudioFor } from "../lib/userAudio";
  import type { SelfState } from "../lib/voice";
  import ChannelSidebar from "./ChannelSidebar.svelte";
  import ChatPane from "./ChatPane.svelte";
  import MemberList from "./MemberList.svelte";
  import ServerRail from "./ServerRail.svelte";
  import SettingsDialog from "./SettingsDialog.svelte";
  import UserMenu from "./UserMenu.svelte";
  import UserPanel from "./UserPanel.svelte";

  let { connection }: { connection: Connection } = $props();

  const server = $derived(connection.state);
  const me = $derived(server.session === null ? undefined : server.users[server.session]);
  const channelId = $derived(me?.channelId ?? 0);
  const channelName = $derived(server.channels[channelId]?.name ?? "");
  const serverName = $derived(server.channels[0]?.name ?? "Mumble");
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

  const isEditable = (t: EventTarget | null) =>
    t instanceof HTMLElement &&
    (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
  const onKey = (e: KeyboardEvent) => {
    if (prefs.mode !== "ptt") return;
    const ev = { code: e.code, repeat: e.repeat, editable: isEditable(e.target) };
    const action = pttAction(e.type as "keydown" | "keyup", ev, prefs.pttKey);
    if (action) capture?.setPttDown(action === "down");
  };

  const updateSelf = (s: SelfState) => {
    if (me) connection.send(setSelfState(me.session, s));
  };
  // Per-user volume and local mute, by user name.
  const audioOf = (name: string) => userAudioFor(userAudio.value, name);
  $effect(() => {
    for (const u of users) connection.playback?.setUserGain(u.session, gainOf(audioOf(u.name)));
  });
  const localMuted = $derived(new Set(users.filter((u) => audioOf(u.name).muted).map((u) => u.session)));

  // Open user menu, and open direct-message conversation (by session).
  let menu = $state<{ session: number; x: number; y: number } | null>(null);
  // The name is kept, so it still shows after the partner left.
  let dm = $state<{ session: number; name: string } | null>(null);
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

  // Unread direct messages: counted per sender, marked read while open.
  let readCounts = $state<Record<number, number>>({});
  const dmCounts = $derived(privateCountsFrom(server.messages, server.session));
  const unread = $derived(unreadCounts(dmCounts, readCounts));
  $effect(() => {
    if (dm === null) return;
    const [session, count] = [dm.session, dmCounts[dm.session] ?? 0];
    untrack(() => {
      if (readCounts[session] !== count) readCounts = { ...readCounts, [session]: count };
    });
  });

  // Same name too: Murmur reuses the session numbers of users who left.
  const dmPartner = $derived.by(() => {
    const user = dm && server.users[dm.session];
    return user && user.name === dm?.name ? user : undefined;
  });
  const send = (text: string) =>
    connection.sendText(dm === null ? { channelId } : { session: dm.session }, text);
  const join = (id: number) => {
    dm = null;
    if (me && id !== channelId) connection.send(joinChannel(me.session, id));
  };
</script>

<svelte:window onkeydown={onKey} onkeyup={onKey} onblur={() => capture?.setPttDown(false)} />

{#if menu && menuUser}
  <UserMenu
    user={menuUser}
    isMe={menuUser.session === server.session}
    audio={audioOf(menuUser.name)}
    x={menu.x}
    y={menu.y}
    onAudio={(patch) => userAudio.update(menuUser.name, patch)}
    onMessage={() => (dm = { session: menuUser.session, name: menuUser.name })}
    onSettings={() => (showSettings = true)}
    onClose={() => (menu = null)}
  />
{/if}

{#if showSettings}
  <SettingsDialog
    settings={prefs}
    level={capture?.level ?? -100}
    onChange={settings.update}
    onClose={() => (showSettings = false)}
  />
{/if}

<div class="layout" class:members={membersOpen}>
  <ServerRail {serverName} />
  <div class="sidebar">
    <header>{serverName}</header>
    <ChannelSidebar
      {server}
      {speaking}
      {localMuted}
      {unread}
      currentChannel={dm === null ? channelId : null}
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
  {#if dm === null}
    <ChatPane
      title={channelName}
      kind="channel"
      messages={messagesIn(server, channelId)}
      welcomeText={channelId === 0 ? server.welcomeText : ""}
      {membersOpen}
      onToggleMembers={() => (membersOpen = !membersOpen)}
      onSend={send}
    />
  {:else}
    <ChatPane
      title={dm.name}
      kind="dm"
      messages={privateMessagesWith(server, dm.session)}
      welcomeText=""
      {membersOpen}
      onToggleMembers={() => (membersOpen = !membersOpen)}
      onSend={dmPartner ? send : undefined}
      onClose={() => (dm = null)}
    />
  {/if}
  {#if membersOpen}
    <MemberList
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
  @media (max-width: 900px) {
    .layout,
    .layout.members {
      grid-template-columns: 72px 240px 1fr;
    }
    .layout > :global(aside) {
      display: none;
    }
  }
</style>
