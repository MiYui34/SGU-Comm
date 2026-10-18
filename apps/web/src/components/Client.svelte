<script lang="ts">
  import {
    type ChatMessage,
    commentMissing,
    inVoice,
    isLobby,
    joinChannel,
    messagesIn,
    escapeHtml,
    queryBans,
    queryRegistered,
    renameRegistered,
    setBans,
    unregister,
    privateMessagesWith,
    queryPermissions,
    registerSelf,
    setSelfState,
    usersSorted,
  } from "@kallisti/protocol";
  import { tick, untrack, type ComponentProps } from "svelte";
  import type { AccountActions } from "../lib/accounts";
  import { createChannelManagement } from "../lib/channelManagement.svelte";
  import { canManageText } from "../lib/channelManagement";
  import type { Connection } from "../lib/connection.svelte";
  import { pictureLookup, pictureUrl, setAvatarPictures } from "../lib/avatar";
  import { customEmojiSet, setCustomEmoji } from "../lib/customEmoji";
  import { isDesktop, registerGlobalPtt } from "../lib/desktop";
  import { menuAnchor } from "../lib/menu";
  import { nextPanel, swipeDirection, togglePins, type Panel, type Side } from "../lib/mobile";
  import { pttAction, pttBlocksDefault } from "../lib/settings";
  import { unreadCounts } from "../lib/chat";
  import { htmlToLines, htmlToText } from "../lib/htmlText";
  import { hideSpoilers } from "../lib/markdown";
  import { parseSearch, type SearchPlace } from "../lib/search";
  import { directChatWith, toChatMessage, type Peer, type TextMessage, type UserTarget } from "../lib/textChannels";
  import { hiddenMembers, offlineMembers } from "../lib/members";
  import { adminAction, moderatorAction, roleOf } from "../lib/roles";
  import { loginAction, mayChangeGuests, mayWrite, readOnlyNote } from "../lib/guests";
  import { missingVoiceApis, unsupportedVoiceMessage } from "../lib/voice/support";
  import { createTextView } from "../lib/textView.svelte";
  import { playPing } from "../lib/ping";
  import { watchAway } from "../lib/away";
  import { statusOfSession } from "../lib/presence";
  import { ownsMessage } from "../lib/messages";
  import { chatKeyOf } from "../lib/composer";
  import type { Place, View } from "../lib/place";
  import {
    mayDeleteMessages,
    moderationRequests,
    moderationRights,
    permissionsToAsk,
    timedOutUntil,
    untilText,
    userModeration,
    type ConfirmKind,
    type ModerationAction,
  } from "../lib/moderation";
  import { notificationBody, notificationPermission, shouldNotify, showNotification } from "../lib/notify";
  import { MENTION_ALL, pingsMe, replyPing } from "../lib/mentions";
  import { blockedUsers, channelPrefs, settings, userAudio } from "../lib/settings.svelte";
  import { mutedIds, notifies, prefOf, withoutMuted } from "../lib/channelPrefs";
  import { previewKey, previewsToAsk } from "../lib/linkPreview";
  import { shouldSendTyping, stoppedTyping, typed, typingNames, typingText, type Typers } from "../lib/typing";
  import { gainOf, userAudioFor } from "../lib/userAudio";
  import { micWanted, type SelfState } from "../lib/voice";
  import ChannelDialog from "./ChannelDialog.svelte";
  import StreamPreview from "./StreamPreview.svelte";
  import StreamStage from "./StreamStage.svelte";
  import ChannelMenu from "./ChannelMenu.svelte";
  import ChannelSidebar from "./ChannelSidebar.svelte";
  import ChatPane from "./ChatPane.svelte";
  import MemberList from "./MemberList.svelte";
  import SearchPanel from "./SearchPanel.svelte";
  import ModerationDialog from "./ModerationDialog.svelte";
  import MembersDialog from "./MembersDialog.svelte";
  import Icon from "./Icon.svelte";
  import ServerRail from "./ServerRail.svelte";
  import SettingsDialog from "./SettingsDialog.svelte";
  import Toast from "./Toast.svelte";
  import UserMenu from "./UserMenu.svelte";
  import UserActionDialog from "./UserActionDialog.svelte";
  import QuickSwitcher, { type Place as SwitcherPlace } from "./QuickSwitcher.svelte";
  import Shortcuts from "./Shortcuts.svelte";
  import { IS_MAC, navAction, stepPlace } from "../lib/switcher";
  import { mirrorIds } from "../lib/mirrors";
  import { tabTitle } from "../lib/title";
  import { messageLink, parseMessageLink } from "../lib/messageLink";
  import { inviteLink, invitePlace, parseInviteLink } from "../lib/invites";
  import UserPanel from "./UserPanel.svelte";
  import VoiceBar from "./VoiceBar.svelte";

  let {
    connection,
    account,
    resume = null,
    onPlace,
    onLogIn,
    site = location.origin,
  }: {
    connection: Connection;
    account?: AccountActions;
    // Reconnects: where to come back to, and where the user is now.
    resume?: Place | null;
    onPlace?: (place: Place) => void;
    // Back to the login screen (a guest that may only read).
    onLogIn?: () => void;
    // The server's address, for message and invite links (the desktop app's
    // own address is not it).
    site?: string;
  } = $props();

  const server = $derived(connection.state);
  const me = $derived(server.session === null ? undefined : server.users[server.session]);
  const channelId = $derived(me?.channelId ?? 0);
  const channelName = $derived(server.channels[channelId]?.name ?? "");
  const serverName = $derived(server.channels[0]?.name ?? "Server");
  const users = $derived(usersSorted(server));
  // Avatars show the pictures of online users, by name.
  const pictures = $derived(new Map(Object.values(server.users).map((u) => [u.name, pictureUrl(u.texture)])));
  const lookup = $derived(pictureLookup(pictures, connection.httpBase));
  // The context is set once: the closure reads the current lookup.
  setAvatarPictures((name, registered) => lookup(name, registered));

  const capture = $derived(connection.capture);
  // Why there is no capture: the browser lacks something.
  const noVoice = unsupportedVoiceMessage(missingVoiceApis());
  // The server does not echo our own voice: add ourselves while sending.
  const speaking = $derived.by(() => {
    const others = connection.playback?.speaking ?? [];
    return new Set(capture?.transmitting && me ? [...others, me.session] : others);
  });

  $effect(() => connection.playback?.setDeaf(me?.selfDeaf ?? false));
  // Outside voice nothing is sent, also while the settings test the mic.
  $effect(() => capture?.setMuted((me?.selfMute ?? false) || !voiceOn));

  let showSettings = $state(false);
  let showSwitcher = $state(false);
  let showShortcuts = $state(false);
  let membersOpen = $state(true);
  // Kallisti servers: root is a lobby, voice channels are joined on purpose.
  const lobby = $derived(isLobby(server));
  const voiceOn = $derived(inVoice(server));
  const prefs = $derived(settings.value);
  $effect(() => capture?.setMode(prefs.mode));
  // A rebound key never sends the keyup of the old one.
  $effect(() => {
    void prefs.pttKey;
    capture?.setPttDown(false);
  });
  $effect(() => capture?.setThreshold(prefs.vadThresholdDb));
  const noiseMode = $derived(prefs.noiseSuppression);
  $effect(() => capture?.setNoiseSuppression(noiseMode));
  // Before the microphone opens (the effect below): no extra reopening.
  const echoCancellation = $derived(prefs.echoCancellation);
  $effect(() => capture?.setEchoCancellation(echoCancellation));
  const musicMode = $derived(prefs.musicMode);
  $effect(() => capture?.setMusicMode(musicMode));
  $effect(() => connection.playback?.setOutputDevice(prefs.outputDeviceId));
  // Opens the microphone while it is used (see micWanted), and reopens it
  // when the device setting changes. Own deriveds: `prefs` is a new object
  // on every settings change, and only a device change may reopen it.
  const inputDeviceId = $derived(prefs.inputDeviceId);
  const closeWhenMuted = $derived(prefs.closeMicWhenMuted);
  const micOn = $derived(
    micWanted({ inVoice: voiceOn, selfMute: me?.selfMute ?? false, settingsOpen: showSettings, closeWhenMuted }),
  );
  $effect(() => {
    const [target, deviceId, on] = [capture, inputDeviceId, micOn];
    untrack(() => (on ? void target?.start(deviceId) : target?.stop()));
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
    if (e.type === "keydown" && navKey(e)) return;
    if (e.type === "keydown" && e.key === "Escape" && phone && panel !== "chat" && !menu && !document.querySelector("dialog[open]")) {
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
  const NO_SWIPE = "dialog, input, textarea, .hold, [role=dialog], [data-no-swipe]";
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
    onHoldTalk: phone && prefs.mode === "ptt" && voiceOn ? (down: boolean) => capture?.setPttDown(down) : undefined,
  });
  const pane = $derived.by(() => {
    if (view.kind === "text")
      return {
        title: textChannel?.name ?? "",
        topic: textChannel?.topic ?? "",
        kind: "text" as const,
        messages: textChat,
        welcomeText: "",
        more: text.more[view.id] ?? false,
        loadingOlder: textView.loadingOlder,
        onLoadOlder: textView.loadOlder,
        error: uploadError ?? text.lastError,
        onUpload: upload,
        uploading,
        uploadMax: text.uploadMax,
        onSend: send,
      } satisfies Partial<ComponentProps<typeof ChatPane>>;
    if (dm === null && voiceChatId !== null)
      return {
        title: channelName,
        topic: htmlToText(server.channels[channelId]?.description ?? ""),
        kind: "channel" as const,
        stored: true,
        messages: textChat,
        welcomeText: "",
        more: text.more[voiceChatId] ?? false,
        loadingOlder: textView.loadingOlder,
        onLoadOlder: textView.loadOlder,
        error: uploadError ?? text.lastError,
        onUpload: upload,
        uploading,
        uploadMax: text.uploadMax,
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
    if (storedDirect !== null)
      return {
        // The partner's current name (renames reach text.directs).
        title: (directChatId !== null && text.directs[directChatId]?.name) || dm.name,
        kind: "dm" as const,
        stored: true,
        messages: textChat,
        welcomeText: "",
        more: directChatId === null ? false : (text.more[directChatId] ?? false),
        loadingOlder: textView.loadingOlder,
        onLoadOlder: textView.loadOlder,
        error: uploadError ?? text.lastError,
        onUpload: directChatId === null ? undefined : upload,
        uploading,
        uploadMax: text.uploadMax,
        onSend: directChatId === null ? undefined : send,
        onClose: () => show({ kind: "voice" }),
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

  // After a reconnect: back where the user was (read once: this client
  // reports its own place right away).
  const back = untrack(() => resume);
  let view = $state<View>(back?.view ?? { kind: "voice" });
  let navigated = back !== null;
  $effect(() =>
    onPlace?.({ view, voice: voiceOn ? channelId : null, selfMute: me?.selfMute ?? false, selfDeaf: me?.selfDeaf ?? false }),
  );
  let restored = false;
  $effect(() => {
    if (restored || !back || !me) return;
    restored = true;
    // Mute first, then into voice: no moment with a live microphone.
    if (back.selfMute || back.selfDeaf) connection.send(setSelfState(me.session, { selfMute: back.selfMute, selfDeaf: back.selfDeaf }));
    if (back.voice !== null && server.channels[back.voice] && me.channelId !== back.voice)
      connection.send(joinChannel(me.session, back.voice));
  });
  const show = (next: View) => {
    jumpTarget = null; // a search result sets it again after this
    navigated = true;
    view = next;
    panel = "chat";
  };
  const dm = $derived(view.kind === "dm" ? view : null);

  // Text channels (Kallisti server only). Like Discord, open the first one
  // when they arrive, unless the user already picked something.
  const text = $derived(connection.text);
  // Statuses (Kallisti server): away after a while without input (sent,
  // not kept), and do not disturb (no notifications or sounds).
  let away = $state(false);
  // Own derived: `text` is a new object on every server event.
  const textReady = $derived(text.available);
  $effect(() => {
    if (!textReady) return;
    const conn = connection;
    away = false;
    return watchAway((on) => {
      away = on;
      conn.setAway(on);
    });
  });
  const dnd = $derived(text.ownStatus.status === "dnd");
  // Custom statuses end by themselves: look again now and then.
  let statusNow = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (statusNow = Date.now()), 30_000);
    return () => clearInterval(timer);
  });
  const statusOf = (session: number) =>
    statusOfSession({ statuses: text.statuses, own: text.ownStatus, me: server.session, away, now: statusNow }, session);
  // Custom server emoji, for messages, reactions and pickers below.
  const customEmoji = $derived(customEmojiSet(text.emoji, connection.httpBase));
  setCustomEmoji(() => customEmoji);
  const textId = $derived(view.kind === "text" ? view.id : null);
  // A voice channel's stored chat (built-in server): the channel that holds
  // it, once the server told us. Not for the lobby.
  const storedVoiceChat = $derived(text.voiceChats && view.kind === "voice" && dm === null && voiceOn && !(lobby && channelId === 0));
  const voiceChatId = $derived(storedVoiceChat ? (text.voiceChannels[channelId] ?? null) : null);
  // Asked once per visit of a channel: after an error, a new visit asks
  // again.
  const askedVoiceChats = new Set<number>();
  $effect(() => {
    void channelId;
    untrack(() => askedVoiceChats.clear());
  });
  $effect(() => {
    if (storedVoiceChat && voiceChatId === null && !askedVoiceChats.has(channelId)) {
      askedVoiceChats.add(channelId);
      connection.requestVoiceChat(channelId);
    }
  });
  // A direct chat with a registered user, both registered (built-in
  // server): stored. Asked for once per conversation opened.
  const storedDirect = $derived(text.voiceChats && me?.userId != null && dm?.userId != null ? dm.userId : null);
  const directChatId = $derived(storedDirect === null ? null : directChatWith(text.directs, storedDirect));
  // An error answers some request: a failed one may be asked again.
  const askedDirects = new Set<number>();
  $effect(() => {
    if (text.lastError) untrack(() => askedDirects.clear());
  });
  $effect(() => {
    if (storedDirect !== null && directChatId === null && !askedDirects.has(storedDirect)) {
      askedDirects.add(storedDirect);
      connection.requestDirect(storedDirect);
    }
  });
  // The stored chat that is open: a text channel, a voice channel's or a
  // direct chat.
  const chatId = $derived(textId ?? voiceChatId ?? directChatId);
  const textChannel = $derived(text.channels.find((c) => c.id === textId));
  // Message links (see lib/messageLink), for text channels and direct
  // chats: open the chat, then jump to the message. false: not here.
  const openMessage = (link: { chat: number; message: number } | null) => {
    if (!link) return false;
    const peer = text.directs[link.chat];
    if (text.channels.some((c) => c.id === link.chat)) show({ kind: "text", id: link.chat });
    else if (peer) openDirect(peer);
    else return false;
    jumpTarget = link.message;
    return true;
  };
  // Opened with a message link: there once the channels are known.
  let linked = untrack(() => parseMessageLink(location.href, site));
  $effect(() => {
    if (!linked || !text.available) return;
    history.replaceState(history.state, "", "/");
    untrack(() => openMessage(linked));
    linked = null;
  });
  // Opened with an invite link (see lib/invites): redeemed once the
  // channels are known; the server says what to open.
  let invited = untrack(() => parseInviteLink(location.href, site));
  $effect(() => {
    const token = invited;
    if (!token || !text.available) return;
    history.replaceState(history.state, "", "/");
    untrack(() => connection.manage({ redeemInvite: token }));
    invited = null;
  });
  // A link made in the members dialog: shown there once. A redeemed link
  // to a voice channel joins it, unless that would leave a call.
  let newInvite = $state<{ voice: number; token: string } | null>(null);
  $effect(() =>
    connection.onTextEvent((ev) => {
      if (ev.type === "invite-created") newInvite = { voice: ev.voice, token: ev.token };
      if (ev.type !== "invite-joined") return;
      const place = invitePlace(ev);
      if (place.kind === "text") show(place);
      else if (voiceOn && !(lobby && channelId === 0)) show({ kind: "voice" });
      else join(place.id);
    }),
  );
  $effect(() => {
    const first = text.channels[0];
    if (first && !navigated && view.kind === "voice") view = { kind: "text", id: first.id };
  });
  // The open channel was removed: to the first text channel, or the voice
  // view.
  $effect(() => {
    if (view.kind !== "text" || !text.available || textChannel) return;
    const first = text.channels[0];
    view = first ? { kind: "text", id: first.id } : { kind: "voice" };
  });
  // Muted text channels show no unread marker (mentions still count).
  // Mutes end on their own: checked every half minute.
  let muteNow = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (muteNow = Date.now()), 30_000);
    return () => clearInterval(timer);
  });
  const mutedText = $derived(mutedIds(channelPrefs.value, connection.httpBase, muteNow));
  // Link previews of the open chat's messages: asked once per message.
  const previewsAsked = new Set<string>();
  $effect(() => {
    if (chatId === null) return;
    const asks = previewsToAsk(text.messages[chatId] ?? [], text.previews, previewsAsked, (url) => !!parseMessageLink(url, site));
    for (const m of asks) previewsAsked.add(previewKey(m));
    const ids = asks.map((m) => m.id);
    for (let i = 0; i < ids.length; i += 30) connection.previews(ids.slice(i, i + 30));
  });
  // Who types where (see lib/typing). typingNow ticks while someone types,
  // so names go away in time.
  let typers = $state.raw<Typers>({});
  let typingNow = $state(Date.now());
  $effect(() =>
    connection.onTextEvent((ev) => {
      if (ev.type === "typing") {
        typingNow = Date.now();
        typers = typed(typers, ev.channel, ev.name, typingNow);
      } else if (ev.type === "message") typers = stoppedTyping(typers, ev.message.channel, ev.message.author);
    }),
  );
  const typingHere = $derived(chatId === null ? [] : typingNames(typers, chatId, typingNow, me?.name ?? "").filter((n) => !blockedUsers.has(n)));
  const anyoneTyping = $derived(typingHere.length > 0);
  // Another channel opened: names that ran out meanwhile go at once.
  $effect(() => {
    void chatId;
    typingNow = Date.now();
  });
  $effect(() => {
    if (!anyoneTyping) return;
    const tick = setInterval(() => (typingNow = Date.now()), 1000);
    return () => clearInterval(tick);
  });
  const typingSent: Record<number, number> = {};
  const onTyping = () => {
    const now = Date.now();
    if (chatId === null || !shouldSendTyping(typingSent[chatId], now)) return;
    typingSent[chatId] = now;
    connection.typing(chatId);
  };
  const textView = createTextView(() => connection, () => chatId, () => me?.name ?? "", blockedUsers.has);
  const unreadText = $derived(withoutMuted(textView.unread, mutedText));
  // Mentions in voice channel chats, by voice channel.
  const mentionVoice = $derived(
    Object.fromEntries(
      Object.entries(text.voiceChannels)
        .map(([voice, chat]) => [voice, textView.mentions[chat] ?? 0])
        .filter(([, n]) => n),
    ),
  );
  // Desktop notifications (see shouldNotify): mentions in stored chats, and
  // private messages.
  const notify = (title: string, body: string, open: boolean, onOpen: () => void) => {
    const visible = document.visibilityState === "visible";
    const enabled = prefs.notifications;
    if (shouldNotify({ enabled, permission: notificationPermission(), visible, deaf: me?.selfDeaf ?? false, dnd, open }))
      showNotification(title, body, onOpen);
  };
  $effect(() =>
    connection.onTextEvent((ev) => {
      if (ev.type !== "message" || !me) return;
      const m = ev.message;
      const own = m.author.toLowerCase() === me.name.toLowerCase();
      if (own || blockedUsers.has(m.author)) return;
      const peer = text.directs[m.channel];
      if (peer) {
        notify(peer.name, notificationBody(hideSpoilers(m.text)), chatId === m.channel, () => openDirect(peer));
        return;
      }
      // By the channel's setting (see lib/channelPrefs); not our own.
      const mentioned = pingsMe(escapeHtml(m.text), m.author, replyPing(m), me.name, m.mentionAll);
      if (!notifies(prefOf(channelPrefs.value, connection.httpBase, m.channel), mentioned, Date.now())) return;
      const textChannel = text.channels.find((c) => c.id === m.channel);
      const voice = Object.entries(text.voiceChannels).find(([, chat]) => chat === m.channel)?.[0];
      const where = textChannel ? `#${textChannel.name}` : (server.channels[Number(voice)]?.name ?? "a voice chat");
      notify(`${m.author || "Server"} in ${where}`, hideSpoilers(m.text).trim().slice(0, 140), chatId === m.channel, () => {
        if (textChannel) show({ kind: "text", id: textChannel.id });
      });
    }),
  );
  let privateSeen: Record<string, number> = untrack(() => ({ ...server.privateCounts })); // counts so far
  $effect(() => {
    const counts = server.privateCounts;
    untrack(() => {
      for (const [peer, n] of Object.entries(counts)) {
        if (n <= (privateSeen[peer] ?? 0) || blockedUsers.has(peer)) continue;
        const last = privateMessagesWith(server, peer).at(-1);
        const user = Object.values(server.users).find((u) => u.name === peer);
        if (last)
          notify(peer, notificationBody(last.text), dm?.name === peer, () => {
            if (user) show({ kind: "dm", session: user.session, name: peer, userId: user.userId });
          });
      }
      privateSeen = { ...counts };
    });
  });
  // A short sound for new mentions outside the open channel (not while
  // deafened or in do not disturb, not in channels set to "nothing").
  let mentionTotal = 0;
  let knownSummary: unknown = null;
  $effect(() => {
    // The settings as they are now; changing them alone plays nothing.
    const channelSettings = untrack(() => channelPrefs.value);
    const quiet = (id: number) => id === chatId || !notifies(prefOf(channelSettings, connection.httpBase, id), true, 0);
    const total = Object.entries(textView.mentions).reduce((a, [id, n]) => (quiet(Number(id)) ? a : a + n), 0);
    // Mentions from before (the summary at login or reconnect): no sound.
    const fresh = text.summary !== knownSummary;
    knownSummary = text.summary;
    if (!fresh && total > mentionTotal && !me?.selfDeaf && !dnd) playPing(prefs.outputDeviceId);
    mentionTotal = total;
  });
  // The search as typed (the server answers with the words only), and
  // chats by name for "in:" (text channels before voice channels before
  // direct chats).
  let searchQuery = $state("");
  const searchPlaces = $derived<SearchPlace[]>([
    ...text.channels.map((c) => ({ name: c.name, kind: "text" as const, id: c.id })),
    ...Object.entries(text.voiceChannels).flatMap(([voice, chat]) => {
      const name = server.channels[Number(voice)]?.name;
      return name ? [{ name, kind: "voice" as const, id: chat }] : [];
    }),
    ...Object.entries(text.directs).map(([chat, peer]) => ({ name: peer.name, kind: "dm" as const, id: Number(chat) })),
  ]);
  const searchChats = $derived(new Map(searchPlaces.toReversed().map((p) => [p.name.toLowerCase(), p.id]))); // the first wins
  // Text channel ids by name, for "#name" links. Derived from the list
  // itself: a new map (and new chat lines) only when the channels change.
  const textChannelList = $derived(text.channels);
  const channelIds = $derived(new Map(textChannelList.map((c) => [c.name.toLowerCase(), c.id])));
  const textChat = $derived(
    chatId === null ? [] : (text.messages[chatId] ?? []).map((m) => toChatMessage(m, connection.httpBase, channelIds)),
  );
  // Server settings (bans, registered users) for moderators and admins.
  let showModeration = $state(false);
  const moderation = $derived({ ...moderationRights(server), emoji: canManageText(server, text) });
  // The right column: members, search results or pins. On phones, search
  // and pins open the right drawer, and closing them closes it.
  let side = $state<Side>("members");
  const openSide = (next: Side) => {
    side = next;
    if (phone) panel = next === "members" ? "chat" : "members";
  };
  // Where a stored message is: "#general", or a voice channel's name.
  const whereOf = (m: TextMessage) => {
    const ch = text.channels.find((c) => c.id === m.channel);
    if (ch) return `#${ch.name}`;
    const peer = text.directs[m.channel];
    if (peer) return `@${peer.name}`;
    const voice = Object.entries(text.voiceChannels).find(([, chat]) => chat === m.channel)?.[0];
    return server.channels[Number(voice)]?.name ?? "a voice chat";
  };
  const resyncs = $derived(text.resyncs); // a number: changes only on a resync
  $effect(() => {
    void resyncs; // a resync forgets them: ask again
    if (side === "pins" && chatId !== null) connection.requestPins(chatId);
  });
  let jumpTarget = $state<number | null>(null);
  const joinedAt = Date.now() - 60_000; // allows for clock differences
  const mine = (m: ChatMessage) => ownsMessage(m, me, joinedAt);
  // Names "@" suggests: who is online, and who wrote in the open chat;
  // for moderators also everyone and here (stored chats, see lib/mentions).
  const people = $derived([
    ...new Set([...users.map((u) => u.name), ...textChat.flatMap((m) => (m.actorName ? [m.actorName] : []))]),
    ...((textId ?? voiceChatId) !== null && canManageText(server, text) ? MENTION_ALL : []),
  ]);
  // One upload at a time. Errors show where server errors show. True:
  // sent (the message box then lets the file go).
  let uploading = $state(false);
  let uploadError = $state<string | null>(null);
  const upload = async (file: File, caption: string, reply?: number, noPing?: boolean) => {
    const channel = chatId;
    if (channel === null || uploading) return false;
    uploading = true;
    uploadError = null;
    try {
      connection.sendToTextChannel(channel, caption, await connection.uploadFile(file), reply, noPing);
      return true;
    } catch (err) {
      uploadError = err instanceof Error ? err.message : String(err);
      return false;
    } finally {
      uploading = false;
    }
  };

  // Registered users who are offline, and members online in channels we
  // cannot see (built-in server), for the member list.
  const offline = $derived(text.voiceChats ? offlineMembers(text.members, text.online, users) : []);
  const hidden = $derived(text.voiceChats ? hiddenMembers(text.members, text.online, users) : []);
  // Open user menu: of a user we see (session) or a member we do not see
  // (user id).
  type MenuAt = { x: number; y: number };
  let menu = $state<((MenuAt & { session: number }) | (MenuAt & { userId: number })) | null>(null);
  const menuUser = $derived(menu && "session" in menu ? server.users[menu.session] : undefined);
  const memberUser: (Peer & { session?: number }) | undefined = $derived.by(() => {
    const m = menu;
    return m && "userId" in m ? (hidden.find((p) => p.id === m.userId) ?? offline.find((p) => p.id === m.userId)) : undefined;
  });
  // A primitive: the bio is asked for once per menu, not on every event.
  const memberMenuId = $derived(memberUser?.id ?? null);
  const openMemberMenu = (userId: number, event: MouseEvent) => {
    event.preventDefault();
    menu = { userId, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
  };
  $effect(() => {
    if (memberMenuId !== null) connection.requestBio(memberMenuId);
  });
  const openMenu = (session: number, event: MouseEvent) => {
    event.preventDefault();
    menu = { session, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
    // Moderation needs our permissions in their channel and where to move them.
    const user = server.users[session];
    if (user && session !== server.session) for (const id of permissionsToAsk(server, text, user)) connection.send(queryPermissions(id));
  };
  // The user left: close their menu (Murmur reuses session numbers). A
  // member's closes when we see them or they are removed.
  $effect(() => {
    if (menu && !menuUser && !memberUser) menu = null;
  });
  // Long comments (bios) load when shown: in the menu, ours in the settings.
  $effect(() => {
    if (menuUser) connection.fetchComment(menuUser.session);
    if (showSettings && me) connection.fetchComment(me.session);
  });
  const menuModeration = $derived(menuUser ? userModeration(server, text, menuUser) : null);
  const moderate = (who: UserTarget, a: ModerationAction) => {
    for (const r of moderationRequests(who, a)) {
      if ("mumble" in r) connection.send(r.mumble);
      else connection.manage(r.manage);
    }
  };
  // A kick, ban, time-out or deleting messages to confirm (deletes: offer
  // deleting messages too); closed when a user we see leaves (the session
  // number may go to someone else).
  type Confirming = { kind: ConfirmKind; who: UserTarget; name: string; timedBans?: boolean; deletes?: boolean };
  let confirming = $state<Confirming | null>(null);
  $effect(() => {
    const who = confirming?.who;
    if (who && "session" in who && server.users[who.session]?.name !== confirming?.name) confirming = null;
  });
  // Our own time-out: the message box says so.
  const myTimeout = $derived(me ? timedOutUntil(text.timeouts, me) : null);
  // A guest that may only read: a way to an account instead of the message
  // box (a guest account gets a password in the settings).
  const guestAccount = $derived(account?.temporary ?? false);
  const readOnly = $derived(!mayWrite(server, text, guestAccount));
  const loginToWrite = $derived(
    readOnly ? { note: readOnlyNote(guestAccount), action: loginAction(guestAccount), onAction: guestAccount ? () => (showSettings = true) : onLogIn } : null,
  );

  // Unread direct messages per sender name, marked read while open.
  let readCounts = $state<Record<string, number>>({});
  const unread = $derived(Object.fromEntries(Object.entries(unreadCounts(server.privateCounts, readCounts)).filter(([name]) => !blockedUsers.has(name))));
  // The tab title: mentions and direct messages not read (see lib/title).
  $effect(() => {
    const sum = (r: Record<string | number, number>) => Object.values(r).reduce((a, n) => a + n, 0);
    document.title = tabTitle(serverName, sum(textView.mentions) + sum(unread));
    return () => (document.title = "Kallisti");
  });
  $effect(() => {
    if (dm === null) return;
    const [name, count] = [dm.name, server.privateCounts[dm.name] ?? 0];
    untrack(() => {
      if (readCounts[name] !== count) readCounts = { ...readCounts, [name]: count };
    });
  });

  // Same name too: Murmur reuses the session numbers of users who left.
  const dmPartner = $derived.by(() => {
    const user = dm && dm.session !== null ? server.users[dm.session] : undefined;
    return user && user.name === dm?.name ? user : undefined;
  });
  // A new message or another view clears an old upload error.
  $effect(() => {
    void view;
    uploadError = null;
  });
  const send = (message: string, reply?: number, noPing?: boolean) => {
    uploadError = null;
    if (chatId !== null) connection.sendToTextChannel(chatId, message, undefined, reply, noPing);
    else if (dm === null) connection.sendText({ channelId }, message);
    else if (dm.session !== null) connection.sendText({ session: dm.session }, message);
  };
  // Screen shares: who is live, and watching (a stream in another channel:
  // join it first).
  const streamClient = $derived(connection.streams);
  const liveSessions = $derived(new Set(text.streams.map((s) => s.session)));
  let pendingWatch = $state<number | null>(null);
  const watchStream = (session: number) => {
    const s = text.streams.find((x) => x.session === session);
    if (!s || !streamClient || session === me?.session) return;
    if (voiceOn && channelId === s.channel) void streamClient.watch(session);
    else {
      pendingWatch = session;
      join(s.channel);
      // Joining may fail (no Enter): then the wish is forgotten.
      setTimeout(() => pendingWatch === session && (pendingWatch = null), 5000);
    }
  };
  // A stream error goes away after a while.
  $effect(() => {
    if (!streamClient?.error) return;
    const t = setTimeout(() => streamClient?.clearError(), 8000);
    return () => clearTimeout(t);
  });
  $effect(() => {
    const s = pendingWatch === null ? undefined : text.streams.find((x) => x.session === pendingWatch);
    if (pendingWatch !== null && (!s || (voiceOn && channelId === s.channel))) {
      untrack(() => {
        if (s) void streamClient?.watch(s.session);
        pendingWatch = null;
      });
    }
  });
  // Streams that ended, or are in a channel we left, are not watched.
  $effect(() => {
    if (!streamClient) return;
    const here = new Set(text.streams.filter((s) => voiceOn && s.channel === channelId).map((s) => s.session));
    untrack(() => {
      for (const s of streamClient.watching) if (!here.has(s)) streamClient.unwatch(s);
    });
  });
  const nameOfSession = (session: number) => server.users[session]?.name ?? "someone";
  $effect(() => {
    streamClient?.setPreviews(prefs.streamPreviews);
  });
  $effect(() => {
    streamClient?.setDeaf(me?.selfDeaf ?? false);
  });
  // A direct chat from the list: the partner's session if online.
  const openDirect = (peer: Peer) => {
    const user = Object.values(server.users).find((u) => u.userId === peer.id);
    show({ kind: "dm", session: user?.session ?? null, name: peer.name, userId: peer.id });
  };
  // Direct chats for the sidebar, the newest first.
  const directList = $derived(
    Object.entries(text.directs)
      .map(([id, peer]) => ({
        channel: Number(id),
        peer,
        latest: Math.max(text.messages[Number(id)]?.at(-1)?.id ?? 0, text.summary[Number(id)]?.latest ?? 0),
      }))
      .sort((a, b) => b.latest - a.latest),
  );
  const join = (id: number) => {
    show({ kind: "voice" });
    if (me && id !== channelId) connection.send(joinChannel(me.session, id));
  };

  // Keyboard (see lib/switcher): Alt+Up/Down through text channels and
  // direct chats (with Shift: unread ones), Ctrl/Cmd+K finds any place.
  // Not over another dialog (the switcher closes itself on Ctrl/Cmd+K).
  const navKey = (e: KeyboardEvent) => {
    const action = navAction(e, IS_MAC);
    if (!action || document.querySelector("dialog[open]")) return false;
    e.preventDefault();
    if (action === "switcher") showSwitcher = true;
    else if (action === "shortcuts") showShortcuts = true;
    else {
      const nav = places.filter((p) => p.kind === "text" || p.key[0] === "d");
      const here = textId !== null ? `t${textId}` : directChatId !== null ? `d${directChatId}` : null;
      const next = stepPlace(
        nav.map((p) => p.key),
        here,
        action.step,
        action.unread ? (key) => !!nav.find((p) => p.key === key)?.unread : undefined,
      );
      nav.find((p) => p.key === next)?.run();
    }
    return true;
  };
  const places = $derived.by((): SwitcherPlace[] => {
    const withChat = new Set(Object.values(text.directs).map((p) => p.id));
    const mirrors = mirrorIds(server, text.channels); // hidden, as in the sidebar
    return [
      ...text.channels.map((c) => ({
        key: `t${c.id}`,
        name: c.name,
        kind: "text" as const,
        unread: unreadText.has(c.id),
        locked: c.mumble !== undefined && c.mumble in text.private,
        run: () => show({ kind: "text", id: c.id }),
      })),
      ...directList.map((d) => ({
        key: `d${d.channel}`,
        name: d.peer.name,
        kind: "dm" as const,
        unread: !!textView.mentions[d.channel],
        run: () => openDirect(d.peer),
      })),
      ...users
        .filter((u) => u.session !== server.session && (u.userId === null || !withChat.has(u.userId)))
        .map((u) => ({ key: `u${u.session}`, name: u.name, kind: "dm" as const, run: () => show({ kind: "dm", session: u.session, name: u.name, userId: u.userId }) })),
      ...Object.values(server.channels)
        .filter((c) => !mirrors.has(c.id) && !(lobby && c.id === 0))
        .map((c) => ({ key: `v${c.id}`, name: c.name, kind: "voice" as const, locked: c.id in text.private, run: () => join(c.id) })),
    ];
  });
  const channels = createChannelManagement(() => connection);

  // Back to the lobby. Its chat is not a place to stay: show a text channel.
  const leaveVoice = () => {
    if (!me || !lobby) return;
    connection.send(joinChannel(me.session, 0));
    const first = text.channels[0];
    if (view.kind === "voice" && first) show({ kind: "text", id: first.id });
  };
</script>

{#if server.notice}
  {#key server.notice.at}<Toast text={server.notice.text} />{/key}
{/if}
{#if text.notice}
  {#key text.notice.n}<Toast text={text.notice.text} />{/key}
{/if}

<svelte:window onkeydown={onKey} onkeyup={onKey} onblur={() => capture?.setPttDown(false)} />
<svelte:document ontouchstart={touchStart} ontouchend={touchEnd} />

{#if channels.menu}
  {@const m = channels.menu}
  <ChannelMenu
    name={m.name}
    x={m.x}
    y={m.y}
    canEdit={m.canEdit}
    canDelete={m.canDelete}
    canMoveUp={m.canMoveUp}
    canMoveDown={m.canMoveDown}
    canMembers={m.canMembers}
    onMembers={() => channels.startMembers(m.kind, m.id)}
    notify={m.kind === "text" ? { pref: prefOf(channelPrefs.value, connection.httpBase, m.id), desktop: prefs.notifications } : undefined}
    onNotify={(patch) => channelPrefs.set(connection.httpBase, m.id, patch)}
    onEdit={() => channels.startEdit(m.kind, m.id)}
    onMove={(step) => channels.move(m.kind, m.id, step)}
    onDelete={() => channels.startDelete(m.kind, m.id)}
    onClose={channels.closeMenu}
  />
{/if}

{#if channels.form}
  {#key channels.form}
    <ChannelDialog form={channels.form.form} onSubmit={channels.submit} onClose={channels.closeForm} />
  {/key}
{/if}

{#if channels.members}
  {@const f = channels.members}
  {#key f.access}
    <MembersDialog
      name={f.kind === "text" ? `#${f.name}` : f.name}
      members={f.members}
      registered={server.registered}
      invites={text.invites[f.access] ?? null}
      link={newInvite?.voice === f.access ? inviteLink(site, newInvite.token) : null}
      onLoad={() => {
        connection.send(queryRegistered());
        connection.manage({ invites: f.access });
      }}
      onSubmit={channels.submitMembers}
      onCreateInvite={(seconds, uses) => connection.manage({ createInvite: f.access, seconds, uses })}
      onDeleteInvite={(id) => connection.manage({ deleteInvite: id })}
      onClose={() => {
        newInvite = null;
        channels.closeMembers();
      }}
    />
  {/key}
{/if}

{#if menu && menuUser}
  <UserMenu
    user={menuUser}
    isMe={menuUser.session === server.session}
    audio={audioOf(menuUser.name)}
    x={menu.x}
    y={menu.y}
    onAudio={(patch) => userAudio.update(menuUser.name, patch)}
    blocked={blockedUsers.has(menuUser.name)}
    onBlock={() => blockedUsers.toggle(menuUser.name)}
    onMessage={readOnly ? undefined : () => show({ kind: "dm", session: menuUser.session, name: menuUser.name, userId: menuUser.userId })}
    onSettings={() => (showSettings = true)}
    role={roleOf(text.roles, menuUser.userId)}
    status={statusOf(menuUser.session)}
    moderator={moderatorAction(server, text, menuUser)}
    onModerator={(on) => menuUser.userId !== null && connection.manage({ moderator: menuUser.userId, on })}
    admin={adminAction(server, text, menuUser)}
    onAdmin={(on) => menuUser.userId !== null && connection.manage({ admin: menuUser.userId, on })}
    moderation={menuModeration}
    timedOutUntil={timedOutUntil(text.timeouts, menuUser)}
    onModerate={(a) => moderate({ session: menuUser.session }, a)}
    onConfirm={(kind) =>
      (confirming = {
        kind,
        who: { session: menuUser.session },
        name: menuUser.name,
        timedBans: menuModeration?.timedBans,
        deletes: menuModeration?.deleteMessages,
      })}
    onDeleteMessages={menuModeration?.deleteMessages
      ? () => {
          // Registered users by id: the dialog stays if they leave.
          const who = menuUser.userId !== null ? { user: menuUser.userId } : { session: menuUser.session };
          confirming = { kind: "delete", who, name: menuUser.name };
        }
      : undefined}
    onClose={() => (menu = null)}
  />
{:else if menu && memberUser}
  {@const p = memberUser}
  <UserMenu
    user={{ name: p.name, comment: text.bios[p.id] ?? "", mute: false, deaf: false }}
    byId
    status={p.session === undefined ? null : statusOf(p.session)}
    isMe={false}
    audio={audioOf(p.name)}
    x={menu.x}
    y={menu.y}
    onAudio={() => {}}
    blocked={blockedUsers.has(p.name)}
    onBlock={() => blockedUsers.toggle(p.name)}
    onMessage={me?.userId != null && !readOnly ? () => openDirect(p) : undefined}
    onSettings={() => {}}
    role={roleOf(text.roles, p.id)}
    moderator={moderatorAction(server, text, { userId: p.id })}
    onModerator={(on) => connection.manage({ moderator: p.id, on })}
    admin={adminAction(server, text, { userId: p.id })}
    onAdmin={(on) => connection.manage({ admin: p.id, on })}
    onDeleteMessages={mayDeleteMessages(server, text, p.id) ? () => (confirming = { kind: "delete", who: { user: p.id }, name: p.name }) : undefined}
    onClose={() => (menu = null)}
  />
{/if}

{#if confirming}
  {@const c = confirming}
  <UserActionDialog
    kind={c.kind}
    name={c.name}
    timedBans={c.timedBans}
    deletes={c.deletes}
    onSubmit={(a) => moderate(c.who, a)}
    onClose={() => (confirming = null)}
  />
{/if}

{#if showSwitcher}
  <QuickSwitcher {places} onClose={() => (showSwitcher = false)} />
{/if}
{#if showShortcuts}
  <Shortcuts onClose={() => (showShortcuts = false)} />
{/if}

{#if showModeration}
  <ModerationDialog
    bans={server.bans}
    registered={server.registered}
    rights={moderation}
    onLoad={(what) => connection.send(what === "bans" ? queryBans() : queryRegistered())}
    onSetBans={(bans) => {
      connection.send(setBans(bans));
      connection.send(queryBans());
    }}
    onRename={(id, name) => {
      connection.send(renameRegistered(id, name));
      connection.send(queryRegistered());
    }}
    onUnregister={(id) => {
      connection.send(unregister(id));
      connection.send(queryRegistered());
    }}
    emoji={customEmoji.list}
    onAddEmoji={connection.uploadEmoji}
    onDeleteEmoji={connection.deleteEmoji}
    guestsReadOnly={mayChangeGuests(server, text) ? text.guestsReadOnly : null}
    onGuestsReadOnly={(on) => connection.manage({ guestsReadOnly: on })}
    onClose={() => (showModeration = false)}
  />
{/if}

{#if showSettings}
  <SettingsDialog
    settings={prefs}
    voiceError={capture ? null : noVoice}
    level={capture?.level ?? -100}
    deviceLabel={capture?.deviceLabel ?? ""}
    fellBack={capture?.fellBack ?? false}
    noiseError={capture?.noiseError ?? null}
    noiseRunning={capture?.running ?? null}
    onChange={settings.update}
    onEcho={capture ? (on) => capture?.setEcho(on) : undefined}
    {account}
    profile={me && {
      name: me.name,
      hasPicture: me.texture !== null || me.textureHash !== null,
      registered: me.userId !== null,
      onSet: connection.setTexture,
    }}
    bio={me && {
      text: htmlToLines(me.comment),
      loading: commentMissing(me),
      onSave: connection.setComment,
    }}
    onClose={() => (showSettings = false)}
  />
{/if}

<div
  class="layout"
  class:members={membersOpen || side !== "members"}
  class:side={side !== "members"}
  data-panel={phone ? panel : undefined}
>
  <div class="left" bind:this={leftEl} inert={phone && panel !== "channels"}>
  <ServerRail {serverName} />
  <div class="sidebar">
    <header>
      <span class="ellipsis">{serverName}</span>
      {#if moderation.any || moderation.emoji}
        <button type="button" class="settings" title="Server settings" aria-label="Server settings" onclick={() => (showModeration = true)}>
          <Icon name="gear" size={18} />
        </button>
      {/if}
    </header>
    <ChannelSidebar
      {server}
      {speaking}
      {localMuted}
      {unread}
      textChannels={text.channels}
      currentText={textId}
      {unreadText}
      {mutedText}
      mentionText={textView.mentions}
      {mentionVoice}
      listenOnly={text.listenOnly}
      privateChannels={text.private}
      live={liveSessions}
      previews={text.streamPreviews}
      onWatch={streamClient ? watchStream : undefined}
      directs={directList}
      currentDirect={directChatId}
      onOpenDirect={openDirect}
      onOpenText={(id) => show({ kind: "text", id })}
      currentChannel={view.kind === "voice" ? channelId : null}
      {lobby}
      onJoin={join}
      onUserMenu={openMenu}
      canCreate={channels.canCreate}
      onCreate={channels.startCreate}
      onChannelMenu={channels.open}
    />
    {#if me}
      {#if lobby && voiceOn}
        {#if streamClient?.preview}
          <StreamPreview stream={streamClient.preview} viewers={streamClient.viewers.map(nameOfSession)} onStop={() => streamClient?.stopSharing()} />
        {/if}
        <VoiceBar
          channel={channelName}
          onLeave={leaveVoice}
          music={musicMode}
          sharing={streamClient?.sharing ?? false}
          onShare={streamClient && text.voiceChats && !phone ? () => (streamClient.sharing ? streamClient.stopSharing() : void streamClient.startSharing()) : undefined}
        />
      {/if}
      <UserPanel
        {me}
        status={!voiceOn ? "Not in voice" : lobby ? "Online" : "Voice connected"}
        active={voiceOn && !lobby}
        micError={micOn ? (capture ? capture.micError : noVoice) : null}
        noVoice={!capture && !!noVoice}
        onSelfState={updateSelf}
        onSettings={() => (showSettings = true)}
        presence={statusOf(me.session).presence}
        own={text.ownStatus}
        now={statusNow}
        onStatus={text.available ? ({ status, ...custom }) => connection.setStatus(status, custom) : undefined}
      />
    {/if}
  </div>
  </div>
  {#if phone && panel !== "chat"}
    <button class="backdrop" aria-label="Close panel" onclick={() => (panel = "chat")}></button>
  {/if}
  {#snippet stage()}
    {#if streamClient}
      <StreamStage
        sessions={streamClient.watching}
        nameOf={nameOfSession}
        attach={streamClient.attach}
        onClose={streamClient.unwatch}
        volumeOf={streamClient.volumeOf}
        onVolume={streamClient.setVolume}
      />
    {/if}
  {/snippet}
  <div class="main" inert={phone && panel !== "chat"}>
    <ChatPane
      {...paneCommon}
      {...pane}
      {...(readOnly ? { onSend: undefined, onUpload: undefined } : {})}
      {loginToWrite}
      chatKey={chatKeyOf(view, channelId)}
      me={me?.name ?? ""}
      {people}
      cannotWrite={myTimeout === null ? null : `You are timed out until ${untilText(myTimeout)}.`}
      mayEdit={mine}
      mayDelete={(m) => mine(m) || canManageText(server, text)}
      onEdit={readOnly ? undefined : connection.editMessage}
      onDelete={connection.deleteMessage}
      onSearch={text.available
        ? (q) => {
            searchQuery = q;
            connection.search(parseSearch(q, searchChats));
            openSide("search");
          }
        : undefined}
      onOpenChannel={(id) => show({ kind: "text", id })}
      stage={streamClient?.watching.length ? stage : undefined}
      error={pane.error ?? streamClient?.error}
      onPins={text.voiceChats && chatId !== null
        ? () => openSide(togglePins(side, !phone || panel === "members"))
        : undefined}
      pinsOpen={side === "pins"}
      mayPin={directChatId !== null || canManageText(server, text)}
      onPin={readOnly ? undefined : connection.pin}
      jumpTarget={chatId !== null && text.loaded[chatId] ? jumpTarget : null}
      onJumped={() => (jumpTarget = null)}
      channelNames={text.channels.map((c) => c.name)}
      {searchPlaces}
      linkTo={chatId !== null && voiceChatId === null ? (id) => messageLink(site, chatId!, id) : undefined}
      onLink={(href) => {
        const link = parseMessageLink(href, site);
        return link && (openMessage(link) ? "opened" : "unavailable");
      }}
      typing={typingText(typingHere)}
      {onTyping}
      onReact={readOnly ? undefined : connection.react}
      myId={me?.userId ?? null}
      previewOf={(id) => text.previews[id] ?? null}
      httpBase={connection.httpBase}
    />
  </div>
  {#if side === "search"}
    <SearchPanel
      inert={phone && panel !== "members"}
      text={text.search ? searchQuery : null}
      messages={(text.search?.messages ?? []).filter((m) => !blockedUsers.has(m.author))}
      where={whereOf}
      canOpen={(m) => text.channels.some((c) => c.id === m.channel) || m.channel in text.directs}
      onOpen={(m) => openMessage({ chat: m.channel, message: m.id })}
      onClose={() => {
        openSide("members");
        document.querySelector<HTMLInputElement>('input[aria-label="Search messages"]')?.focus();
      }}
    />
  {:else if side === "pins" && chatId !== null}
    <SearchPanel
      inert={phone && panel !== "members"}
      text={text.pins[chatId] ? "" : null /* null: loading */}
      heading="Pinned messages"
      label="Pinned messages"
      none="No pinned messages here yet."
      messages={(text.pins[chatId] ?? []).filter((m) => !blockedUsers.has(m.author))}
      where={whereOf}
      canOpen={() => true}
      onOpen={(m) => {
        jumpTarget = m.id;
        panel = "chat";
      }}
      onClose={() => openSide("members")}
    />
  {:else if membersOpen || phone}
    <MemberList
      inert={phone && panel !== "members"}
      {channelName}
      {unread}
      {speaking}
      onUserMenu={openMenu}
      roleOf={(u) => roleOf(text.roles, u.userId)}
      {statusOf}
      {offline}
      {hidden}
      onMemberMenu={openMemberMenu}
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
  header .settings {
    margin-left: auto;
    color: var(--text-muted);
  }
  header .settings:hover {
    color: var(--text);
  }
  header {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    gap: 8px;
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
    /* Members do not fit; search and pins slide over the chat. */
    .layout {
      position: relative;
    }
    .layout:not(.side) > :global(aside) {
      display: none;
    }
    .layout.side > :global(aside) {
      position: absolute;
      top: 0;
      right: 0;
      bottom: 0;
      z-index: 20;
      width: min(320px, 40vw);
      box-shadow: -4px 0 16px rgb(0 0 0 / 0.3);
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
