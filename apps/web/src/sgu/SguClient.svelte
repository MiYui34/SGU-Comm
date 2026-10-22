<script lang="ts">
  import { joinChannel, setSelfState } from "@kallisti/protocol";
  import type { Connection } from "../lib/connection.svelte";
  import { myChannelOf, myNameOf, voiceRooms } from "./fromServer";
  import Shell from "./Shell.svelte";
  import "./theme.css";
  import type { ConnectionStats, SguState } from "./types";

  // 把真实的 Mumble 连接接到中文外壳上：房间是语音频道，人是频道里的人，
  // 延迟用这条连接自己的 ping，进出房间就是移动频道。
  let { connection }: { connection: Connection } = $props();

  const server = $derived(connection.state);
  const rooms = $derived(voiceRooms(server));
  const name = $derived(myNameOf(server));
  const me = $derived(server.session === null ? null : (server.users[server.session] ?? null));

  // 根频道（0）是没语音的大厅，界面上等于「不在任何房间」。
  const here = $derived(myChannelOf(server));
  const joinedId = $derived(here === "0" || here === null ? null : here);

  // 延迟就是这条语音连接自己的往返，不用再向服务器另外发请求。
  const stats = $derived<ConnectionStats | null>(
    connection.info.rtt === null
      ? null
      : {
          rtt: Math.round(connection.info.rtt),
          jitter: connection.info.spread ?? 0,
          loss: connection.info.bad ?? 0,
        },
  );

  const controls = $derived(
    me === null
      ? null
      : {
          micOn: !me.selfMute,
          soundOn: !me.selfDeaf,
          onToggleMic: () =>
            connection.send(setSelfState(me.session, { selfMute: !me.selfMute, selfDeaf: me.selfDeaf })),
          onToggleSound: () =>
            connection.send(setSelfState(me.session, { selfMute: me.selfMute, selfDeaf: !me.selfDeaf })),
        },
  );

  // 人数、版本由外壳自己去问游戏服；这里给空，免得没配的时候显示示例数据。
  const snapshot = $derived<SguState>({
    server: { online: 0, slots: 0, version: "" },
    me: { name: name ?? "我", initial: (name ?? "我").slice(0, 1), linked: false },
    rooms,
    joinedRoomId: joinedId,
    players: [],
    connection: stats ?? { rtt: 0, jitter: 0, loss: 0 },
    activity: [],
  });

  function join(id: string) {
    const session = server.session;
    if (session === null) return;
    connection.send(joinChannel(session, Number(id)));
  }

  function leave() {
    const session = server.session;
    if (session === null) return;
    connection.send(joinChannel(session, 0));
  }
</script>

<div class="sgu-host">
  <Shell
    {snapshot}
    {rooms}
    joinedRoomId={joinedId}
    {stats}
    {controls}
    onJoin={join}
    onLeave={leave}
    chat={connection.text}
    onSendText={(id, body) => connection.sendText({ channelId: id }, body)}
    onOpenChat={(id) => connection.requestHistory(id)}
  />
</div>

<style>
  /* 接进真实客户端时铺满整屏（App.svelte 用 ?ui=sgu 切过来）。 */
  .sgu-host {
    position: fixed;
    inset: 0;
    z-index: 40;
  }
</style>
