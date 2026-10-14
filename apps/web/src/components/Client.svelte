<script lang="ts">
  import {
    joinChannel,
    messagesIn,
    setSelfState,
    usersSorted,
  } from "@mumble-discord/protocol";
  import type { Connection } from "../lib/connection.svelte";
  import type { SelfState } from "../lib/voice";
  import ChannelSidebar from "./ChannelSidebar.svelte";
  import ChatPane from "./ChatPane.svelte";
  import MemberList from "./MemberList.svelte";
  import ServerRail from "./ServerRail.svelte";
  import UserPanel from "./UserPanel.svelte";

  let { connection }: { connection: Connection } = $props();

  const state = $derived(connection.state);
  const me = $derived(state.session === null ? undefined : state.users[state.session]);
  const channelId = $derived(me?.channelId ?? 0);
  const channelName = $derived(state.channels[channelId]?.name ?? "");
  const serverName = $derived(state.channels[0]?.name ?? "Mumble");
  const users = $derived(usersSorted(state));

  const NOBODY: ReadonlySet<number> = new Set();
  const speaking = $derived(connection.playback?.speaking ?? NOBODY);

  $effect(() => connection.playback?.setDeaf(me?.selfDeaf ?? false));

  const join = (id: number) => {
    if (me && id !== channelId) connection.send(joinChannel(me.session, id));
  };
  const updateSelf = (s: SelfState) => {
    if (me) connection.send(setSelfState(me.session, s));
  };
  const send = (text: string) => connection.sendText(channelId, text);
</script>

<div class="layout">
  <ServerRail {serverName} />
  <div class="sidebar">
    <header>{serverName}</header>
    <ChannelSidebar {state} {speaking} currentChannel={channelId} onJoin={join} />
    {#if me}<UserPanel {me} onSelfState={updateSelf} />{/if}
  </div>
  <ChatPane
    {channelName}
    messages={messagesIn(state, channelId)}
    welcomeText={channelId === 0 ? state.welcomeText : ""}
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
