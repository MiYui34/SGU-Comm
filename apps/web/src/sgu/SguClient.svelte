<script lang="ts">
  import {
    createChannel,
    joinChannel,
    may,
    Perm,
    queryBans,
    queryRegistered,
    renameRegistered,
    setBans,
    moderateUser,
    setSelfState,
    unregister,
  } from "@kallisti/protocol";
  import type { Connection } from "../lib/connection.svelte";
  import ModerationDialog from "../components/ModerationDialog.svelte";
  import UserActionDialog from "../components/UserActionDialog.svelte";
  import SettingsDialog from "../components/SettingsDialog.svelte";
  import Shortcuts from "../components/Shortcuts.svelte";
  import { mayChangeGuests } from "../lib/guests";
  import { moderationRequests, moderationRights, userModeration } from "../lib/moderation";
  import { parseSearch } from "../lib/search";
  import { settings } from "../lib/settings.svelte";
  import { myChannelOf, myNameOf, voicePeople, voiceRooms } from "./fromServer";
  import Shell from "./Shell.svelte";
  import "./theme.css";
  import type { ConnectionStats, PersonAction, SguState } from "./types";

  // 把真实的 Mumble 连接接到中文外壳上：房间是语音频道，人是频道里的人，
  // 延迟用这条连接自己的 ping，进出房间就是移动频道。
  let { connection }: { connection: Connection } = $props();

  const server = $derived(connection.state);
  const rooms = $derived(voiceRooms(server));
  const people = $derived(voicePeople(server));
  const name = $derived(myNameOf(server));
  const me = $derived(server.session === null ? null : (server.users[server.session] ?? null));
  // 建频道要看服务器给的权限位，没有就不显示那个入口。
  const canCreate = $derived(may(server, 0, Perm.MakeChannel));

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
    me: { name: name ?? "我", initial: (name ?? "我").slice(0, 1) },
    rooms,
    joinedRoomId: joinedId,
    players: [],
    connection: stats ?? { rtt: 0, jitter: 0, loss: 0 },
    activity: [],
  });

  // 搜索框里的 `from:某某` / `in:#频道` 交给上游的解析器认，认不出的
  // 频道名按它的规矩变成 -1（搜所有频道）。
  function searchChats(): Map<string, number> {
    const text = connection.text;
    if (text === null) return new Map();
    const chats = new Map<string, number>();
    for (const channel of text.channels) chats.set(channel.name.toLowerCase(), channel.id);
    for (const [voice, chat] of Object.entries(text.voiceChannels)) {
      const name = server.channels[Number(voice)]?.name ?? voice;
      chats.set(name.toLowerCase(), chat);
    }
    for (const [id, peer] of Object.entries(text.directs)) chats.set(peer.name.toLowerCase(), Number(id));
    return chats;
  }

  // 设置与键位表是上游现成的两个弹窗，直接挂上来。
  let prefsOpen = $state(false);
  let keysOpen = $state(false);
  const prefs = $derived(settings.value);
  const level = $derived(connection.capture?.level ?? -100);

  // 服务器管理：封禁、注册用户、访客只读、验证码，权限位由服务器给。
  const text = $derived(connection.text);
  const rights = $derived(moderationRights(server));
  let modOpen = $state(false);
  // 正在确认的那个人（踢出/封禁/禁言要再问一次）。
  let asking = $state<{ session: number; name: string; kind: "kick" | "ban" | "timeout" } | null>(null);

  // 玩家页每一行能做什么，按权限和对方身份算。
  function personActions(session: number): PersonAction[] {
    const user = server.users[session];
    const held = text;
    if (user === undefined || held === null) return [];
    const m = userModeration(server, held, user, null);
    if (m === null) return [];
    const out: PersonAction[] = [];
    if (m.mute) {
      out.push({
        key: "mute",
        label: user.mute ? "取消静音" : "静音",
        run: () => connection.send(moderateUser(session, { mute: !user.mute })),
      });
    }
    if (m.kick) out.push({ key: "kick", label: "踢出", run: () => (asking = { session, name: user.name, kind: "kick" }) });
    if (m.ban) out.push({ key: "ban", label: "封禁", run: () => (asking = { session, name: user.name, kind: "ban" }) });
    if (m.timeout) {
      out.push({ key: "timeout", label: "禁言一段时间", run: () => (asking = { session, name: user.name, kind: "timeout" }) });
    }
    return out;
  }

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

  // 新房间挂在根频道下面，和服务器自带的那几个平级。
  function create(name: string) {
    connection.send(createChannel(0, name));
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
    {people}
    {canCreate}
    onCreate={create}
    chat={connection.text}
    onSendText={(id, body, extra) =>
      connection.sendToTextChannel(id, body, extra?.upload, extra?.reply)}
    onUploadText={(file) => connection.uploadFile(file)}
    canWriteText={(id) => may(server, id, Perm.Write)}
    onReactText={(id, emoji, on) => connection.react(id, emoji, on)}
    onEditText={(id, body) => connection.editMessage(id, body)}
    onPinText={(id, on) => connection.pin(id, on)}
    myId={me?.userId ?? null}
    onOpenChat={(id) => connection.requestHistory(id)}
    onSearchText={(query) => connection.search(parseSearch(query, searchChats()))}
    onPinsText={(id) => connection.requestPins(id)}
    onDeleteText={(id) => connection.deleteMessage(id)}
    onCreateThread={(id, name) => connection.thread({ create: id, name })}
    onOpenThread={(id) => connection.thread({ get: id })}
    onOpenSettings={() => (prefsOpen = true)}
    onOpenShortcuts={() => (keysOpen = true)}
    canModerateServer={rights.any}
    {personActions}
    onOpenModeration={() => (modOpen = true)}

  />
</div>

{#if prefsOpen}
  <SettingsDialog
    settings={prefs}
    {level}
    onChange={settings.update}
    onEcho={connection.capture ? (on: boolean) => connection.capture?.setEcho(on) : undefined}
    onClose={() => (prefsOpen = false)}
  />
{/if}

{#if keysOpen}
  <Shortcuts onClose={() => (keysOpen = false)} />
{/if}

{#if asking !== null}
  <UserActionDialog
    kind={asking.kind}
    name={asking.name}
    onSubmit={(action) => {
      const who = asking;
      if (who === null) return;
      for (const request of moderationRequests({ session: who.session }, action)) {
        if ("mumble" in request) connection.send(request.mumble);
        else connection.manage(request.manage);
      }
      asking = null;
    }}
    onClose={() => (asking = null)}
  />
{/if}

{#if modOpen && text !== null}
  <ModerationDialog
    bans={server.bans}
    registered={server.registered}
    rights={{ ...rights, emoji: may(server, 0, Perm.MakeChannel) }}
    onLoad={(what) => connection.send(what === "bans" ? queryBans() : queryRegistered())}
    onSetBans={(bans) =>
      connection.sendAwait([setBans(bans), queryBans()], (state) => state.bans?.length === bans.length)}
    onRename={(id, name) =>
      connection.sendAwait([renameRegistered(id, name), queryRegistered()], (state) =>
        state.registered?.some((user) => user.userId === id && user.name === name) === true,
      )}
    onUnregister={(id) =>
      connection.sendAwait([unregister(id), queryRegistered()], (state) =>
        state.registered?.some((user) => user.userId === id) === false,
      )}
    emoji={[]}
    onAddEmoji={connection.uploadEmoji}
    onDeleteEmoji={connection.deleteEmoji}
    guestsReadOnly={mayChangeGuests(server, text) ? text.guestsReadOnly : null}
    onGuestsReadOnly={(on) => connection.request({ guestsReadOnly: on })}
    captcha={text.captcha}
    onCaptcha={(level) => connection.request({ captcha: level })}
    onClose={() => (modOpen = false)}
  />
{/if}

<style>
  /* 接进真实客户端时铺满整屏（App.svelte 用 ?ui=sgu 切过来）。 */
  .sgu-host {
    position: fixed;
    inset: 0;
    z-index: 40;
  }
</style>
