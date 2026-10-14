<script lang="ts">
  import {
    joinChannel,
    messagesIn,
    setSelfState,
    usersSorted,
  } from "@mumble-discord/protocol";
  import { untrack } from "svelte";
  import type { Connection } from "../lib/connection.svelte";
  import { pttAction } from "../lib/settings";
  import { settings } from "../lib/settings.svelte";
  import type { SelfState } from "../lib/voice";
  import ChannelSidebar from "./ChannelSidebar.svelte";
  import ChatPane from "./ChatPane.svelte";
  import MemberList from "./MemberList.svelte";
  import ServerRail from "./ServerRail.svelte";
  import SettingsDialog from "./SettingsDialog.svelte";
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

  const join = (id: number) => {
    if (me && id !== channelId) connection.send(joinChannel(me.session, id));
  };
  const updateSelf = (s: SelfState) => {
    if (me) connection.send(setSelfState(me.session, s));
  };
  const send = (text: string) => connection.sendText(channelId, text);
</script>

<svelte:window onkeydown={onKey} onkeyup={onKey} onblur={() => capture?.setPttDown(false)} />

{#if showSettings}
  <SettingsDialog
    settings={prefs}
    level={capture?.level ?? -100}
    onChange={settings.update}
    onClose={() => (showSettings = false)}
  />
{/if}

<div class="layout">
  <ServerRail {serverName} />
  <div class="sidebar">
    <header>{serverName}</header>
    <ChannelSidebar state={server} {speaking} currentChannel={channelId} onJoin={join} />
    {#if me}
      <UserPanel
        {me}
        micError={capture?.micError ?? null}
        onSelfState={updateSelf}
        onSettings={() => (showSettings = true)}
      />
    {/if}
  </div>
  <ChatPane
    {channelName}
    messages={messagesIn(server, channelId)}
    welcomeText={channelId === 0 ? server.welcomeText : ""}
    onSend={send}
  />
  <MemberList
    {channelName}
    here={users.filter((u) => u.channelId === channelId)}
    elsewhere={users.filter((u) => u.channelId !== channelId)}
  />
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: 72px 240px 1fr 240px;
    height: 100%;
  }
  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-sidebar);
  }
  header {
    display: flex;
    align-items: center;
    height: 48px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border);
    font-weight: 600;
  }
  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 72px 240px 1fr;
    }
    .layout > :global(aside) {
      display: none;
    }
  }
</style>
