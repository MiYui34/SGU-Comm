<script lang="ts">
  import {
    type ChatMessage,
    commentMissing,
    inVoice,
    isLobby,
    isTextOnly,
    joinChannel,
    listedUsers,
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
    voiceElsewhere,
  } from "@kallisti/protocol";
  import { tick, untrack, type ComponentProps } from "svelte";
  import type { AccountActions } from "../lib/accounts";
  import { createChannelManagement } from "../lib/channelManagement.svelte";
  import { canManageText, canModerate } from "../lib/channelManagement";
  import type { Connection } from "../lib/connection.svelte";
  import { onlinePicture, pictureLookup, setAvatarPictures } from "../lib/avatar";
  import { customEmojiSet, setCustomEmoji } from "../lib/customEmoji";
  import { isDesktop, registerGlobalPtt } from "../lib/desktop";
  import { menuAnchor } from "../lib/menu";
  import { nextPanel, swipeDirection, toggleSide, type Panel, type Side } from "../lib/mobile";
  import { pttAction, pttBlocksDefault } from "../lib/settings";
  import { unreadCounts } from "../lib/chat";
  import { htmlToLines, htmlToText } from "../lib/htmlText";
  import { hideSpoilers } from "../lib/markdown";
  import { parseSearch, type SearchPlace } from "../lib/search";
  import { directChatWith, toChatMessage, type DisplayMessage, type Peer, type TextMessage, type UserTarget } from "../lib/textChannels";
  import { hiddenMembers, offlineMembers, pictureNames, readerGroups } from "../lib/members";
  import { adminAction, badgeOf, moderatorAction, roleOf } from "../lib/roles";
  import { loginAction, mayChangeGuests, mayWrite, readOnlyNote } from "../lib/guests";
  import { missingVoiceApis, unsupportedVoiceMessage } from "../lib/voice/support";
  import { createTextView } from "../lib/textView.svelte";
  import { playPing } from "../lib/ping";
  import { allowSound, playSound, snapshotOf, voiceSound, type VoiceSnapshot } from "../lib/sounds";
  import { watchAway } from "../lib/away";
  import { statusOfSession } from "../lib/presence";
  import { ownsMessage } from "../lib/messages";
  import { lastTurnAt } from "../lib/slowMode";
  import { chatKeyOf, type Drafts } from "../lib/composer";
  import type { Place, View } from "../lib/place";
  import { rejoinTarget } from "../lib/reconnect";
  import { voiceSession } from "../lib/mediaSession";
  import {
    channelTimedOutUntil,
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
  import { chatSeen, notificationBody, notifier, shouldNotify } from "../lib/notify";
  import { createNotifyLeader } from "../lib/notifyLeader";
  import { MENTION_ALL, pingsMe, replyPing } from "../lib/mentions";
  import { blockedUsers, channelPrefs, collapsedCategories, directPrefs, settings, userAudio } from "../lib/settings.svelte";
  import { textOrder } from "../lib/categories";
  import { forgetPref, isMuted, mutedIds, notifies, prefOf, pushLists, setThreadPref, withoutMuted, withParents } from "../lib/channelPrefs";
  import { closeDirect, quietDirects, reopenDirect, togglePin, visibleDirects } from "../lib/directList";
  import { browserPushSupport, syncPush, type PushPrefs } from "../lib/push";
  import { previewKey, previewsToAsk } from "../lib/linkPreview";
  import { shouldSendTyping, stoppedTyping, typed, typingNames, typingText, type Typers } from "../lib/typing";
  import { gainOf, userAudioFor } from "../lib/userAudio";
  import { loadSelfState, micWanted, saveSelfState, type SelfState } from "../lib/voice";
  import { listenOnlySessions } from "../lib/listenOnly";
  import ChannelDialog from "./ChannelDialog.svelte";
  import StreamPreview from "./StreamPreview.svelte";
  import StreamStage from "./StreamStage.svelte";
  import { shareSources } from "../lib/stream/client.svelte";
  import { ownerOf } from "../lib/stream/protocol";
  import { KINDS, qualityOptions, type Kind, type Quality } from "../lib/stream/quality";
  import { goLiveEntries, isLive, streamBlock, type GoLiveAction } from "../lib/stream/goLive";
  import ChannelMenu from "./ChannelMenu.svelte";
  import ChannelSidebar from "./ChannelSidebar.svelte";
  import ChatPane from "./ChatPane.svelte";
  import ThreadList from "./ThreadList.svelte";
  import ThreadNameDialog from "./ThreadNameDialog.svelte";
  import { activeThreads, chatTarget, defaultThreadName, startedIn, threadList, threadRequest, type Thread } from "../lib/threads";
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
  import ObsDialog from "./ObsDialog.svelte";
  import { mayRecord } from "../lib/recording";

  let {
    connection,
    account,
    resume = null,
    onPlace,
    onLogIn,
    site = location.origin,
    drafts = { value: new Map() },
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
    // Unsent texts of the chats not open (see chatCommon): kept by the
    // caller, so that they outlive a reconnect.
    drafts?: Drafts;
  } = $props();

  const server = $derived(connection.state);
  const me = $derived(server.session === null ? undefined : server.users[server.session]);
  // A text-only session (our account is in voice elsewhere) is in no
  // channel: it shows root.
  const textOnly = $derived(isTextOnly(server));
  const channelId = $derived(me && !textOnly ? me.channelId : 0);
  const channelName = $derived(server.channels[channelId]?.name ?? "");
  const serverName = $derived(server.channels[0]?.name ?? "Server");
  const users = $derived(listedUsers(server));
  const elsewhere = $derived(voiceElsewhere(server));
  // Avatars show the pictures of online users, by name.
  const pictureBase = $derived(connection.text.avatars ? connection.httpBase : null);
  const pictures = $derived(new Map(Object.values(server.users).map((u) => [u.name, onlinePicture(u, pictureBase)])));
  // Offline members: the saved picture, only of those who have one.
  const saved = $derived(pictureNames(connection.text.members, connection.text.pictures));
  const lookup = $derived(pictureLookup(pictures, saved, connection.httpBase));
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
  $effect(() => connection.playback?.setBuffer(prefs.playbackBuffer));
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
  // A stored chat's pane fields (null: not known yet). Errors: not the
  // quiet ones (quietError).
  const storedPane = (id: number | null) => ({
    more: id !== null && (text.more[id] ?? false),
    loadingOlder: textView.loadingOlder(id),
    onLoadOlder: () => textView.loadOlder(id),
    error: uploadError ?? (text.quietError ? null : text.lastError),
    onUpload: id === null ? undefined : (file: File, caption: string, reply?: number, noPing?: boolean) => upload(file, caption, reply, noPing, id),
    uploading,
    uploadMax: text.uploadMax,
    returned: id !== null && text.returned?.channel === id ? text.returned : null,
  });
  const pane = $derived.by(() => {
    if (view.kind === "text")
      return {
        title: textChannel?.name ?? "",
        topic: textChannel?.topic ?? "",
        kind: "text" as const,
        messages: textChat,
        welcomeText: "",
        ...storedPane(chatId),
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
        ...storedPane(chatId),
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
        ...storedPane(chatId),
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
  // Per-user volume and local mute, by user name. Playback drops a
  // speaker's voice until this set it.
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
  // The voice channel we come back to counts as ours until we are in it
  // (or it failed): a second drop meanwhile keeps it, and so does the
  // media session.
  let rejoining = $state(back?.voice ?? null);
  const voiceHere = $derived(rejoining ?? (voiceOn ? channelId : null));
  $effect(() => {
    if (rejoining !== null && me?.channelId === rejoining) rejoining = null;
  });
  // The session that holds our voice: the old one until we are back in it.
  const placeSession = $derived(rejoining !== null ? (back?.session ?? null) : server.session);
  $effect(() =>
    onPlace?.({
      session: placeSession,
      view,
      voice: voiceHere,
      thread: threadId,
      selfMute: me?.selfMute ?? false,
      selfDeaf: me?.selfDeaf ?? false,
    }),
  );
  // In voice: keeps phones from freezing the page (see lib/mediaSession).
  const session = $derived(voiceHere === null ? null : voiceSession({ channel: server.channels[voiceHere]?.name ?? "", server: serverName, lobby }));
  $effect(() =>
    connection.playback?.setVoice(session, (action) => (action === "leave" ? leaveVoice() : updateSelf({ selfMute: true, selfDeaf: true }))),
  );
  $effect(() => () => connection.playback?.releaseVoice());
  // Mute and deafen sounds (see sounds.ts), not for about 3 s after a
  // reconnect: we restore our mute and deafen then. REJOIN_MS: how long
  // the rejoin may take.
  const REJOIN_MS = 5000;
  let soundState: VoiceSnapshot | null = null;
  let lastSound = -Infinity;
  let restored = false;
  const storage = typeof localStorage === "undefined" ? undefined : localStorage;
  $effect(() => {
    if (restored || !me) return;
    restored = true;
    // A reconnect keeps the state it had, a new page the saved one.
    const self = back ?? loadSelfState(storage, site);
    lastSound = performance.now() + 2500;
    // Mute first, then into voice: no moment with a live microphone.
    if (self.selfMute || self.selfDeaf) connection.send(setSelfState(me.session, { selfMute: self.selfMute, selfDeaf: self.selfDeaf }));
    if (!back) return;
    const target = rejoinTarget(back, me.channelId, (id) => !!server.channels[id], elsewhere?.session ?? null);
    if (target === null) return void (rejoining = null);
    connection.send(joinChannel(me.session, target));
    setTimeout(() => (rejoining = null), REJOIN_MS); // refused: not in voice after all
  });
  // Saved once the state above is restored (it starts unmuted).
  $effect(() => {
    const self = { selfMute: me?.selfMute ?? false, selfDeaf: me?.selfDeaf ?? false };
    if (me && untrack(() => restored)) saveSelfState(storage, site, self);
  });
  $effect(() => {
    const next = snapshotOf(server);
    untrack(() => {
      const sound = next && voiceSound(soundState, next);
      soundState = next;
      if (!sound || !prefs.sounds || !allowSound(lastSound, performance.now())) return;
      lastSound = performance.now();
      playSound(sound, prefs.outputDeviceId, prefs.soundVolume);
    });
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
  const listenOnlyUsers = $derived(listenOnlySessions(server, text.listenOnly));
  $effect(() => connection.playback?.setListenOnly(listenOnlyUsers));
  // Recorded voice channels; staff record and download (see lib/recording).
  const recorded = $derived(new Set(text.recording ?? []));
  const staffRecords = $derived(mayRecord(server, text));
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
  // An open thread (see below), and a message to jump to there.
  let threadId = $state<number | null>(null);
  let threadJump = $state<number | null>(null);
  const textChannel = $derived(text.channels.find((c) => c.id === textId));
  // The Mumble channel of what we look at (a text channel's mirror, or our
  // voice channel; null: a direct chat): its moderators and time-outs.
  const viewMumble = $derived(textId !== null ? (textChannel?.mumble ?? null) : view.kind === "voice" && dm === null ? channelId : null);
  // Message links (see lib/messageLink), for text channels, threads and
  // direct chats: open the chat, then jump to the message. A chat we do
  // not know may be an archived thread: asked for, opened when it comes
  // (see openThread). false: not here.
  let askedThread: { id: number; message?: number } | null = null;
  const openChat = (chat: number, message?: number) => {
    const to = chatTarget(chat, text);
    if (!to) return false;
    if ("thread" in to) openThread(to.thread, message);
    else if ("ask" in to) {
      askedThread = { id: chat, message };
      connection.thread({ get: chat });
    } else {
      if ("channel" in to) show({ kind: "text", id: chat });
      else openDirect(to.direct);
      jumpTarget = message ?? null;
    }
    return true;
  };
  const openMessage = (link: { chat: number; message: number } | null) => !!link && openChat(link.chat, link.message);
  // After a reconnect: the thread open before, again (once the channels
  // are known; an archived one is asked for).
  let reopen = untrack(() => back?.thread ?? null);
  $effect(() => {
    if (reopen === null || !text.available) return;
    const id = reopen;
    reopen = null;
    untrack(() => openChat(id));
  });
  // Push notifications (see lib/push), for accounts with a password: on
  // start and when channel settings or blocks change, the server learns
  // this session, the quiet chats (see pushLists) and the blocked users
  // (as a string: the same prefs post nothing). A click on one in an open
  // window comes from the service worker.
  const pushName = $derived(account && !account.temporary && browserPushSupport() !== "none" ? me?.name : undefined);
  const pushPrefsKey = $derived(
    JSON.stringify({
      ...pushLists(channelPrefs.value, connection.httpBase, Object.keys(text.directs).map(Number)),
      blocked: [...blockedUsers.list].sort(),
    }),
  );
  const pushPrefs = $derived(JSON.parse(pushPrefsKey) as PushPrefs);
  $effect(() => {
    if (pushName && browserPushSupport() === "ok") void syncPush(connection.httpBase, pushName, pushPrefs);
  });
  $effect(() => {
    const sw = navigator.serviceWorker;
    const onMessage = (e: MessageEvent) => {
      if (e.data?.type === "open" && typeof e.data.url === "string") openMessage(parseMessageLink(e.data.url, site));
    };
    sw?.addEventListener("message", onMessage);
    return () => sw?.removeEventListener("message", onMessage);
  });
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
  // Voice moved to another device: its chat is not ours any more.
  $effect(() => {
    if (!textOnly) return;
    untrack(() => {
      const first = text.channels[0];
      if (view.kind === "voice" && first) show({ kind: "text", id: first.id });
    });
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
  // The settings as they hold: threads without their own take their
  // channel's (see lib/channelPrefs).
  const chatPrefs = $derived(withParents(channelPrefs.value, connection.httpBase, text.threads));
  const prefIn = (id: number) => prefOf(chatPrefs, connection.httpBase, id);
  const mutedText = $derived(mutedIds(chatPrefs, connection.httpBase, muteNow));
  // Link previews of the open chat's messages: asked once per message.
  const previewsAsked = new Set<string>();
  $effect(() => {
    const asks = [chatId, threadId].flatMap((id) =>
      id === null ? [] : previewsToAsk(text.messages[id] ?? [], text.previews, previewsAsked, (url) => !!parseMessageLink(url, site)),
    );
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
  const typingIn = (id: number | null) =>
    id === null ? [] : typingNames(typers, id, typingNow, me?.name ?? "").filter((n) => !blockedUsers.has(n));
  const typingHere = $derived(typingIn(chatId));
  const typingThread = $derived(typingIn(threadId));
  const anyoneTyping = $derived(typingHere.length + typingThread.length > 0);
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
  const typingTo = (id: number | null) => () => {
    const now = Date.now();
    if (id === null || !shouldSendTyping(typingSent[id], now)) return;
    typingSent[id] = now;
    connection.typing(id);
  };
  const onTyping = $derived(typingTo(chatId));
  // Only a visible tab marks new messages read (see createTextView).
  let pageVisible = $state(document.visibilityState === "visible");
  const textView = createTextView(
    () => connection,
    () => (phone && threadId !== null ? [threadId] : [chatId, threadId]),
    () => me?.name ?? "",
    blockedUsers.has,
    () => pageVisible,
  );
  const unreadText = $derived(withoutMuted(textView.unread, mutedText));
  // Unread counts as shown (muted direct chats count none).
  const mentions = $derived(quietDirects(textView.mentions, text.directs, mutedText));
  // Mentions in voice channel chats, by voice channel.
  const mentionVoice = $derived(
    Object.fromEntries(
      Object.entries(text.voiceChannels)
        .map(([voice, chat]) => [voice, textView.mentions[chat] ?? 0])
        .filter(([, n]) => n),
    ),
  );
  // Desktop notifications (see shouldNotify): mentions in stored chats, and
  // private messages. Made now: the desktop app reads its permission once.
  const notifications = notifier();
  // Of several tabs of the account in this browser, one notifies (see
  // lib/notifyLeader).
  let leader: ReturnType<typeof createNotifyLeader> | null = null;
  const myName = $derived(me?.name);
  $effect(() => {
    if (!myName) return;
    const tabs = (leader = createNotifyLeader(`kallisti-notify ${connection.httpBase} ${myName.toLowerCase()}`));
    const used = () => document.visibilityState === "visible" && tabs.touch();
    window.addEventListener("focus", used);
    document.addEventListener("visibilitychange", used);
    window.addEventListener("pagehide", tabs.close);
    return () => {
      window.removeEventListener("focus", used);
      document.removeEventListener("visibilitychange", used);
      window.removeEventListener("pagehide", tabs.close);
      tabs.close();
      leader = null;
    };
  });
  const notify = (title: string, body: string, open: boolean, onOpen: () => void) => {
    const visible = chatSeen({ visible: document.visibilityState === "visible", focused: document.hasFocus(), desktop: isDesktop() });
    const enabled = prefs.notifications;
    if (leader?.leads() !== false && shouldNotify({ enabled, permission: notifications.permission(), visible, deaf: me?.selfDeaf ?? false, dnd, open }))
      notifications.show(title, body, onOpen);
  };
  $effect(() =>
    connection.onTextEvent((ev) => {
      if (ev.type !== "message" || !me) return;
      const m = ev.message;
      const own = m.author.toLowerCase() === me.name.toLowerCase();
      if (own || blockedUsers.has(m.author)) return;
      const peer = text.directs[m.channel];
      if (peer) {
        if (isMuted(prefIn(m.channel), Date.now())) return;
        notify(peer.name, notificationBody(hideSpoilers(m.text)), chatId === m.channel, () => openDirect(peer));
        return;
      }
      // By the chat's setting (see lib/channelPrefs); not our own.
      const mentioned = pingsMe(escapeHtml(m.text), m.author, replyPing(m), me.name, m.mentionAll);
      if (!notifies(prefIn(m.channel), mentioned, Date.now())) return;
      const textChannel = text.channels.find((c) => c.id === m.channel);
      const t = text.threads[m.channel];
      notify(`${m.author || "Server"} in ${whereOf(m)}`, hideSpoilers(m.text).trim().slice(0, 140), chatId === m.channel || threadId === m.channel, () => {
        if (textChannel) show({ kind: "text", id: textChannel.id });
        else if (t) openThread(t);
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
    const channelSettings = untrack(() => chatPrefs);
    const quiet = (id: number) => {
      const pref = prefOf(channelSettings, connection.httpBase, id);
      return id === chatId || id === threadId || (id in text.directs ? isMuted(pref, Date.now()) : !notifies(pref, true, 0));
    };
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
    if (next !== "members") threadId = null; // the column shows it now
    if (phone) panel = next === "members" ? "chat" : "members";
  };
  // Where a stored message is: "#general", or a voice channel's name.
  const whereOf = (m: TextMessage): string => {
    const t = text.threads[m.channel];
    if (t) return `${whereOf({ ...m, channel: t.channel })} › ${t.name}`;
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
  // Slow mode in the open text channel (staff are exempt).
  // A thread has its channel's slow mode, and its own turns; starting one
  // takes a turn in the channel.
  const slowIn = (id: number | null, chat: readonly ChatMessage[]) =>
    id === null || !textChannel?.slowMode || canModerate(server, text, viewMumble)
      ? null
      : { seconds: textChannel.slowMode, lastPost: lastTurnAt(chat, mine, startedIn(text.threads, id, me, joinedAt)), until: text.slowUntil[id] };
  const slow = $derived(slowIn(textId, textChat));
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
  const upload = async (file: File, caption: string, reply?: number, noPing?: boolean, channel = chatId) => {
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
  // In a private text or voice channel: who reads it (null: not private).
  const privateMembers = $derived.by(() => {
    const access = textId !== null ? textChannel?.mumble : view.kind === "voice" ? channelId : undefined;
    return access === undefined ? undefined : text.private[access];
  });
  const readers = $derived(
    privateMembers && readerGroups(privateMembers, text.roles, text.members, text.online, users, me?.userId ?? null),
  );
  // Open user menu: of a user we see (session) or a member we do not see
  // (user id).
  type MenuAt = { x: number; y: number };
  let menu = $state<((MenuAt & { session: number }) | (MenuAt & { userId: number })) | null>(null);
  const menuUser = $derived(menu && "session" in menu ? server.users[menu.session] : undefined);
  const memberUser: (Peer & { session?: number }) | undefined = $derived.by(() => {
    const m = menu;
    if (!m || !("userId" in m)) return undefined;
    // A direct chat's partner may be in neither list (not a member here).
    const find = (list: Peer[]) => list.find((p) => p.id === m.userId);
    return find(hidden) ?? find(offline) ?? find(Object.values(text.directs));
  });
  // A primitive: the bio is asked for once per menu, not on every event.
  const memberMenuId = $derived(memberUser?.id ?? null);
  // @mention pills: the names that become pills (online users and members),
  // and a click on one opens that user's menu (online first, else the member).
  const knownNames = $derived([...new Set([...users.map((u) => u.name), ...Object.values(text.members)])]);
  const openMentionMenu = (name: string, pill: HTMLElement, event: MouseEvent) => {
    const key = name.toLowerCase();
    const at = menuAnchor(event, pill.getBoundingClientRect());
    const user = users.find((u) => u.name.toLowerCase() === key);
    if (user) return userMenuAt(user.session, at);
    const member = Object.entries(text.members).find(([, n]) => n.toLowerCase() === key);
    if (member) menu = { userId: Number(member[0]), ...at };
  };
  const openMemberMenu = (userId: number, event: MouseEvent) => {
    event.preventDefault();
    menu = { userId, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
  };
  $effect(() => {
    if (memberMenuId !== null) connection.requestBio(memberMenuId);
  });
  const openMenu = (session: number, event: MouseEvent) => {
    event.preventDefault();
    userMenuAt(session, menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()));
  };
  const userMenuAt = (session: number, at: { x: number; y: number }) => {
    menu = { session, ...at };
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
  const menuModeration = $derived(menuUser ? userModeration(server, text, menuUser, viewMumble) : null);
  // A Mumble channel's name for people: "#general" for a text channel's
  // mirror.
  const mumbleName = (id: number) => {
    const t = text.channels.find((c) => c.mumble === id);
    return t ? `#${t.name}` : (server.channels[id]?.name ?? "");
  };
  const moderate = (who: UserTarget, a: ModerationAction) => {
    for (const r of moderationRequests(who, a)) {
      if ("mumble" in r) connection.send(r.mumble);
      else connection.manage(r.manage);
    }
  };
  // A kick, ban, time-out or deleting messages to confirm (deletes: offer
  // deleting messages too); closed when a user we see leaves (the session
  // number may go to someone else).
  // channel: a channel time-out there.
  type Confirming = { kind: ConfirmKind; who: UserTarget; name: string; timedBans?: boolean; deletes?: boolean; channel?: number };
  let confirming = $state<Confirming | null>(null);
  $effect(() => {
    const who = confirming?.who;
    if (who && "session" in who && server.users[who.session]?.name !== confirming?.name) confirming = null;
  });
  // Our own time-out: the message box says so.
  const myTimeout = $derived(me ? timedOutUntil(text.timeouts, me) : null);
  // Who may become a private channel's member: staff ask Mumble for the
  // registered users; a channel's moderators may not, and pick from the
  // members list (registered users but guest accounts).
  const registeredUsers = $derived(
    canManageText(server, text) ? server.registered : Object.entries(text.members).map(([id, name]) => ({ userId: Number(id), name })),
  );
  const myChannelTimeout = $derived(me ? channelTimedOutUntil(server, text.channelTimeouts, me, viewMumble) : null);
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
    document.title = tabTitle(serverName, sum(mentions) + sum(unread));
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

  // What both chat panes (the open chat and an open thread) share.
  const chatCommon = $derived({
    drafts,
    loginToWrite,
    me: me?.name ?? "",
    people,
    knownNames,
    onMention: openMentionMenu,
    cannotWrite:
      myTimeout !== null
        ? `You are timed out until ${untilText(myTimeout)}.`
        : myChannelTimeout !== null
          ? `You are timed out in this channel until ${untilText(myChannelTimeout)}.`
          : null,
    mayEdit: mine,
    mayDelete: (m: DisplayMessage) => mine(m) || canModerate(server, text, viewMumble),
    onEdit: readOnly ? undefined : connection.editMessage,
    onDelete: connection.deleteMessage,
    onOpenChannel: (id: number) => show({ kind: "text", id }),
    onPin: readOnly ? undefined : connection.pin,
    channelNames: text.channels.map((c) => c.name),
    onLink: (href: string) => {
      const link = parseMessageLink(href, site);
      return link && (openMessage(link) ? "opened" : "unavailable");
    },
    onReact: readOnly ? undefined : connection.react,
    myId: me?.userId ?? null,
    previewOf: (id: number) => text.previews[id] ?? null,
    httpBase: connection.httpBase,
  } satisfies Partial<ComponentProps<typeof ChatPane>>);

  // Threads (see lib/threads), the open one: next to its channel (full
  // screen on phones). One we started opens when the server made it.
  const thread = $derived(threadId === null ? undefined : text.threads[threadId]);
  const threadAside = $derived(!phone && !!thread);
  const openThread = (t: Thread, jump?: number) => {
    if (textId !== t.channel) show({ kind: "text", id: t.channel });
    threadId = t.id;
    threadJump = jump ?? null;
    panel = "chat";
  };
  // Gone, or its channel is not open: closed.
  $effect(() => {
    if (threadId !== null && (!thread || thread.channel !== textId)) threadId = null;
  });
  // One asked for by id (see openChat) opens when it comes (or the server
  // says it is not there).
  let startedFrom: number | null = null;
  $effect(() =>
    connection.onTextEvent((ev) => {
      if (ev.type === "thread" && ev.thread.start === startedFrom) {
        startedFrom = null;
        openThread(ev.thread);
      } else if (ev.type === "thread" && ev.thread.id === askedThread?.id) {
        const jump = askedThread.message;
        askedThread = null;
        openThread(ev.thread, jump);
      } else if (ev.type === "no-thread" && ev.id === askedThread?.id) askedThread = null;
    }),
  );
  const threadByStart = $derived(new Map(Object.values(text.threads).map((t) => [t.start, t])));
  const threadChat = $derived(
    threadId === null ? [] : (text.messages[threadId] ?? []).map((m) => toChatMessage(m, connection.httpBase, channelIds)),
  );
  // A name to give: a new thread from a message, or a rename.
  let naming = $state<{ create: number; name: string } | { rename: number; name: string } | null>(null);
  // Rights as the server has them: its maker renames and archives it, the
  // channel's moderators also delete it.
  const threadRights = (t: Thread) => {
    const mod = canModerate(server, text, text.channels.find((c) => c.id === t.channel)?.mumble ?? null);
    const own = me?.userId != null && t.authorId === me.userId && !readOnly;
    return { change: own || mod, remove: mod };
  };
  let threadMenu = $state<(MenuAt & { id: number }) | null>(null);
  const openThreadMenu = (id: number, event: MouseEvent) => {
    event.preventDefault();
    threadMenu = { id, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
  };
  // A channel's archived threads, a page at a time (asked: the page asked
  // for, until it comes).
  let archivedAsked = $state<string | null>(null);
  const loadArchived = (channel: number) => {
    const before = text.archived[channel]?.threads.at(-1)?.id;
    archivedAsked = `${channel}:${before ?? 0}`;
    connection.thread({ archived: channel, before });
  };
  $effect(() => {
    void [text.archived, text.lastError]; // answered, or refused
    archivedAsked = null;
  });
  $effect(() => {
    if (side === "threads" && textId !== null && !text.archived[textId] && archivedAsked === null) untrack(() => loadArchived(textId!));
  });
  // Screen shares and cameras (by stream id): who is live, and watching (a
  // stream in another channel: join it first).
  const streamClient = $derived(connection.streams);
  const shareable = $derived(shareSources(phone));
  const liveSessions = $derived(new Set(text.streams.map((s) => s.session)));
  let pendingWatch = $state<number | null>(null);
  const watchStream = (session: number) => {
    const s = text.streams.find((x) => x.session === session);
    if (!s || !streamClient || ownerOf(session) === me?.session) return;
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
  // A user's name, also from a stream id.
  const nameOfSession = (session: number) => server.users[ownerOf(session)]?.name ?? "someone";
  $effect(() => {
    streamClient?.setPreviews(prefs.streamPreviews);
  });
  $effect(() => {
    streamClient?.setCameraDevice(prefs.cameraDeviceId);
  });
  $effect(() => {
    for (const kind of KINDS) streamClient?.setQuality(kind, prefs.streamQuality[kind]);
  });
  const setStreamQuality = (kind: Kind, q: Quality) => settings.update({ streamQuality: { ...prefs.streamQuality, [kind]: q } });
  const toggleStream = (kind: Kind) => (streamClient?.sharing[kind] ? streamClient.stopSharing(kind) : void streamClient?.startSharing(kind));
  // The "Go live" menu. A stream from OBS is our screen share on the server;
  // OBS needs an account with a password and the server's WHIP.
  let whip = $state(false);
  let showObs = $state(false);
  $effect(() => {
    if (account && !account.temporary) void account.streamKey().then((k) => (whip = k.enabled));
  });
  const goLive = $derived.by(() => {
    const sharing = streamClient?.sharing ?? { screen: false, camera: false };
    const obsLive = !sharing.screen && me !== undefined && liveSessions.has(me.session);
    const state = {
      sharing,
      obsLive,
      sources: streamClient && text.voiceChats ? shareable : { screen: false, camera: false },
      whip,
      password: !!account && !account.temporary,
      blocked: me ? streamBlock(me, server.channelPermissions[channelId]) : null,
    };
    return { entries: goLiveEntries(state), live: isLive(state), obsLive };
  });
  const onGoLive = (action: GoLiveAction) => (action === "obs" ? (showObs = true) : toggleStream(action));
  // The camera switch shows with more than one camera.
  let cameraCount = $state(0);
  $effect(() => {
    if (!streamClient?.sharing.camera) return;
    void navigator.mediaDevices
      .enumerateDevices()
      .then((list) => (cameraCount = list.filter((d) => d.kind === "videoinput").length))
      .catch(() => {});
  });
  $effect(() => {
    streamClient?.setDeaf(me?.selfDeaf ?? false);
  });
  // A direct chat from the list: the partner's session if online.
  const openDirect = (peer: Peer) => {
    const user = Object.values(server.users).find((u) => u.userId === peer.id);
    show({ kind: "dm", session: user?.session ?? null, name: peer.name, userId: peer.id });
  };
  // Direct chats for the sidebar: pinned ones, then the newest first;
  // closed ones not (see lib/directList).
  const directState = $derived(directPrefs.of(connection.httpBase));
  const directList = $derived(
    visibleDirects(
      Object.entries(text.directs).map(([id, peer]) => ({
        channel: Number(id),
        peer,
        latest: Math.max(text.messages[Number(id)]?.at(-1)?.id ?? 0, text.summary[Number(id)]?.latest ?? 0),
        pinned: directState.pinned.includes(Number(id)),
      })),
      directState,
    ),
  );
  // Opening a closed chat (from a user menu, a link) lists it again.
  $effect(() => {
    const id = directChatId;
    if (id !== null) untrack(() => directPrefs.update(connection.httpBase, (p) => reopenDirect(p, id)));
  });
  // The right-click menu of a direct chat.
  let directMenu = $state<(MenuAt & { channel: number }) | null>(null);
  const openDirectMenu = (channel: number, event: MouseEvent) => {
    event.preventDefault();
    directMenu = { channel, ...menuAnchor(event, (event.currentTarget as Element).getBoundingClientRect()) };
  };
  // The partner's user menu: after this menu closed (it gives focus back).
  const openPeerMenu = async (peer: Peer, at: MenuAt) => {
    await tick();
    const user = Object.values(server.users).find((u) => u.userId === peer.id);
    menu = user ? { session: user.session, x: at.x, y: at.y } : { userId: peer.id, x: at.x, y: at.y };
  };
  // Closed: read, and gone from the list until a newer message. The open
  // one: show another place first.
  const closeDirectChat = (channel: number) => {
    if (channel === directChatId) show(text.channels[0] ? { kind: "text", id: text.channels[0].id } : { kind: "voice" });
    const latest = directList.find((d) => d.channel === channel)?.latest ?? 0;
    textView.markRead(channel);
    directPrefs.update(connection.httpBase, (p) => closeDirect(p, channel, latest));
  };
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
    // Closed chats too (after the listed ones): opening one shows it again.
    const listed = new Set(directList.map((d) => d.channel));
    const closed = Object.entries(text.directs)
      .filter(([id]) => !listed.has(Number(id)))
      .map(([id, peer]) => ({ channel: Number(id), peer }));
    const directs = [...directList, ...closed];
    const withChat = new Set(directs.map((d) => d.peer.id));
    const mirrors = mirrorIds(server, text.channels, text.categories); // hidden, as in the sidebar
    const categoryName = (id?: number) => text.categories.find((c) => c.id === id)?.name;
    return [
      // In the sidebar's order (Alt+Up/Down follows it).
      ...textOrder(text.channels, text.categories).flatMap((c) => [
        {
          key: `t${c.id}`,
          name: c.name,
          kind: "text" as const,
          category: categoryName(c.category),
          unread: unreadText.has(c.id),
          locked: c.mumble !== undefined && c.mumble in text.private,
          run: () => show({ kind: "text", id: c.id }),
        },
        ...activeThreads(text.threads, c.id).map((t) => ({
          key: `h${t.id}`,
          name: t.name,
          kind: "thread" as const,
          category: `#${c.name}`,
          unread: unreadText.has(t.id),
          run: () => openThread(t),
        })),
      ]),
      ...directs.map((d) => ({
        key: `d${d.channel}`,
        name: d.peer.name,
        kind: "dm" as const,
        unread: !!mentions[d.channel],
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
<svelte:document ontouchstart={touchStart} ontouchend={touchEnd} onvisibilitychange={() => (pageVisible = document.visibilityState === "visible")} />

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
    canModerators={m.canModerators}
    category={m.kind === "category"}
    canCreateSub={m.canCreateSub}
    recording={m.kind === "voice" && staffRecords ? recorded.has(m.id) : undefined}
    onRecord={(on) => connection.manage({ record: m.id, on })}
    onMembers={() => channels.startMembers(m.kind, m.id)}
    onModerators={() => channels.startModerators(m.kind, m.id)}
    notify={m.kind === "text" ? { pref: prefOf(channelPrefs.value, connection.httpBase, m.id), desktop: prefs.notifications } : undefined}
    onNotify={(patch) => channelPrefs.set(connection.httpBase, m.id, patch)}
    onEdit={() => channels.startEdit(m.kind, m.id)}
    onCreateSub={() => channels.startCreate("voice", m.id)}
    onMove={(step) => channels.move(m.kind, m.id, step)}
    onDelete={() => channels.startDelete(m.kind, m.id)}
    onClose={channels.closeMenu}
  />
{/if}

{#if threadMenu && text.threads[threadMenu.id]}
  {@const tm = threadMenu}
  {@const t = text.threads[tm.id]!}
  {@const rights = threadRights(t)}
  <ChannelMenu
    name={t.name}
    x={tm.x}
    y={tm.y}
    canEdit={rights.change}
    canDelete={rights.remove}
    onEdit={() => (naming = { rename: t.id, name: t.name })}
    onDelete={() => confirm(`Delete the thread “${t.name}” and its messages?`) && connection.thread({ remove: t.id })}
    notify={{ pref: prefIn(t.id), desktop: prefs.notifications }}
    onNotify={(patch) => channelPrefs.update((p) => setThreadPref(p, connection.httpBase, t.id, prefIn(t.id), patch))}
    thread={{
      following: text.follows.has(t.id),
      onFollow: () => connection.thread({ follow: t.id, on: !text.follows.has(t.id) }),
      archived: t.archived === true,
      onArchive: rights.change ? () => connection.thread({ archive: t.id, on: !t.archived }) : undefined,
      onInherit: t.id in (channelPrefs.value[connection.httpBase] ?? {})
        ? () => channelPrefs.update((p) => forgetPref(p, connection.httpBase, t.id))
        : undefined,
    }}
    onClose={() => (threadMenu = null)}
  />
{/if}

{#if naming}
  {@const n = naming}
  <ThreadNameDialog
    title={"create" in n ? "Create thread" : "Rename thread"}
    name={n.name}
    action={"create" in n ? "Create thread" : "Rename"}
    onSubmit={(name) => {
      if ("create" in n) startedFrom = n.create;
      connection.thread("create" in n ? { create: n.create, name } : { rename: n.rename, name });
    }}
    onClose={() => (naming = null)}
  />
{/if}

{#if directMenu && text.directs[directMenu.channel]}
  {@const d = directMenu}
  {@const peer = text.directs[d.channel]!}
  <ChannelMenu
    name={peer.name}
    x={d.x}
    y={d.y}
    notify={{ pref: prefOf(channelPrefs.value, connection.httpBase, d.channel), desktop: prefs.notifications }}
    onNotify={(patch) => channelPrefs.set(connection.httpBase, d.channel, patch)}
    direct={{
      pinned: directState.pinned.includes(d.channel),
      onProfile: () => openPeerMenu(peer, d),
      // Not the shown counts: a muted chat may be unread too.
      onRead: textView.mentions[d.channel] ? () => textView.markRead(d.channel) : undefined,
      onPin: () => directPrefs.update(connection.httpBase, (p) => togglePin(p, d.channel)),
      onHide: () => closeDirectChat(d.channel),
    }}
    onClose={() => (directMenu = null)}
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
      registered={registeredUsers}
      invites={text.invites[f.access] ?? null}
      link={newInvite?.voice === f.access ? inviteLink(site, newInvite.token) : null}
      onLoad={() => {
        if (canManageText(server, text)) connection.send(queryRegistered());
        connection.manage({ invites: f.access });
      }}
      onSubmit={channels.submitMembers}
      onCreateInvite={(seconds, uses) => connection.request({ createInvite: f.access, seconds, uses })}
      onDeleteInvite={(id) => connection.request({ deleteInvite: id })}
      onClose={() => {
        newInvite = null;
        channels.closeMembers();
      }}
    />
  {/key}
{/if}

{#if channels.moderators}
  {@const f = channels.moderators}
  {#key f.access}
    <MembersDialog
      moderators
      name={f.kind === "text" ? `#${f.name}` : f.name}
      members={f.members}
      registered={server.registered}
      onLoad={() => connection.send(queryRegistered())}
      onSubmit={channels.submitModerators}
      onClose={channels.closeModerators}
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
    channelTimeout={menuModeration?.channelTimeout != null
      ? {
          name: mumbleName(menuModeration.channelTimeout),
          // That channel's own (the server ends only those).
          until: timedOutUntil(text.channelTimeouts.filter((t) => t.channel === menuModeration.channelTimeout), menuUser),
        }
      : null}
    onModerate={(a) => moderate({ session: menuUser.session }, a)}
    onConfirm={(kind) =>
      (confirming = {
        kind: kind === "channelTimeout" ? "timeout" : kind,
        who: { session: menuUser.session },
        name: menuUser.name,
        timedBans: menuModeration?.timedBans,
        deletes: kind !== "channelTimeout" && menuModeration?.deleteMessages,
        ...(kind === "channelTimeout" && menuModeration?.channelTimeout != null ? { channel: menuModeration.channelTimeout } : {}),
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
    where={c.channel === undefined ? undefined : mumbleName(c.channel)}
    onSubmit={(a) => moderate(c.who, c.channel !== undefined && a.kind === "timeout" ? { kind: "channelTimeout", channel: c.channel, seconds: a.seconds } : a)}
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
      // Done once the list asked for again has the new length.
      const before = server.bans;
      return connection.sendAwait([setBans(bans), queryBans()], (s) => s.bans !== before && s.bans?.length === bans.length);
    }}
    onRename={(id, name) =>
      connection.sendAwait([renameRegistered(id, name), queryRegistered()], (s) => s.registered?.find((u) => u.userId === id)?.name === name)}
    onUnregister={(id) =>
      connection.sendAwait([unregister(id), queryRegistered()], (s) => !!s.registered && !s.registered.some((u) => u.userId === id))}
    emoji={customEmoji.list}
    onAddEmoji={connection.uploadEmoji}
    onDeleteEmoji={connection.deleteEmoji}
    guestsReadOnly={mayChangeGuests(server, text) ? text.guestsReadOnly : null}
    onGuestsReadOnly={(on) => connection.request({ guestsReadOnly: on })}
    captcha={text.captcha}
    onCaptcha={(level) => connection.request({ captcha: level })}
    connections={text.connections}
    onPollConnections={mayChangeGuests(server, text) ? () => connection.manage({ connections: true }) : undefined}
    messageRules={text.messageRules}
    onLoadMessageRules={mayChangeGuests(server, text) ? () => connection.manage({ messageRules: true }) : undefined}
    onSaveMessageRules={(rules) => connection.request({ setMessageRules: rules })}
    threadArchive={text.threadArchive}
    onThreadArchive={canManageText(server, text)
      ? (seconds) => connection.ask(threadRequest({ archiveAfter: seconds }), (ev) => ev.type === "thread-archive" && ev.seconds === seconds)
      : undefined}
    recordings={text.recordings}
    recording={text.recording}
    onLoadRecordings={staffRecords ? () => connection.manage({ recordings: true }) : undefined}
    onDownloadRecording={connection.downloadRecording}
    onClose={() => (showModeration = false)}
  />
{/if}

{#if showObs && account}
  <ObsDialog {account} live={goLive.obsLive} onClose={() => (showObs = false)} />
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
    push={pushName ? { base: connection.httpBase, name: pushName, prefs: pushPrefs } : undefined}
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
  class:members={membersOpen || side !== "members" || threadAside}
  class:side={side !== "members" || threadAside}
  class:thread={threadAside}
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
      categories={text.categories}
      collapsedCategories={collapsedCategories.of(connection.httpBase)}
      onToggleCategory={(id) => collapsedCategories.toggle(connection.httpBase, id)}
      canCreateIn={channels.canCreateIn}
      currentText={textId}
      {unreadText}
      {mutedText}
      mentionText={mentions}
      {mentionVoice}
      listenOnly={text.listenOnly}
      privateChannels={text.private}
      recording={recorded}
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
      onDirectMenu={openDirectMenu}
      threadsOf={(id) => activeThreads(text.threads, id)}
      follows={text.follows}
      currentThread={threadId}
      onOpenThread={openThread}
      onThreadMenu={openThreadMenu}
    />
    {#if me}
      {#if lobby && voiceOn}
        {#if streamClient}
          {#each KINDS as kind (kind)}
            {@const stream = streamClient.preview[kind]}
            {#if stream}
              <StreamPreview
                {stream}
                {kind}
                viewers={streamClient.viewers[kind].map(nameOfSession)}
                quality={prefs.streamQuality[kind]}
                options={qualityOptions(streamClient.limits)}
                onQuality={(q) => setStreamQuality(kind, q)}
                onSwitch={kind === "camera" && cameraCount > 1 ? streamClient.switchCamera : undefined}
                onStop={() => streamClient?.stopSharing(kind)}
              />
            {/if}
          {/each}
        {/if}
        <VoiceBar
          channel={channelName}
          recording={recorded.has(channelId)}
          onLeave={leaveVoice}
          music={musicMode}
          info={connection.info}
          live={goLive.live}
          entries={goLive.entries}
          {onGoLive}
        />
      {:else if elsewhere}
        <VoiceBar
          channel={server.channels[elsewhere.channelId]?.name ?? ""}
          onLeave={leaveVoice}
          info={connection.info}
          onJoinHere={() => join(elsewhere.channelId)}
        />
      {/if}
      <UserPanel
        {me}
        status={elsewhere ? "In voice on another device" : !voiceOn ? "Not in voice" : lobby ? "Online" : "Voice connected"}
        active={voiceOn && !lobby}
        info={voiceOn && !lobby ? connection.info : undefined}
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
        {phone}
      />
    {/if}
  {/snippet}
  {#snippet threadPane(t: Thread)}
    {@const parent = text.channels.find((c) => c.id === t.channel)}
    <ChatPane
      {...paneCommon}
      onMenu={phone ? paneCommon.onMenu : undefined}
      compact
      title={t.name}
      topic={parent ? `#${parent.name}` : ""}
      kind="text"
      messages={threadChat}
      welcomeText=""
      {...storedPane(t.id)}
      onSend={(message, reply, noPing) => {
        uploadError = null;
        connection.sendToTextChannel(t.id, message, undefined, reply, noPing);
      }}
      {...chatCommon}
      {...(readOnly ? { onSend: undefined, onUpload: undefined } : {})}
      chatKey="thread {t.id}"
      slow={slowIn(t.id, threadChat)}
      mayPin={canModerate(server, text, viewMumble)}
      jumpTarget={text.loaded[t.id] ? threadJump : null}
      onJumped={() => (threadJump = null)}
      linkTo={(id) => messageLink(site, t.id, id)}
      typing={typingText(typingThread)}
      onTyping={typingTo(t.id)}
      onClose={() => (threadId = null)}
    >
      {#snippet intro()}
        <div class="thread-intro">
          <h2>{t.name}</h2>
          <p>Thread started by {t.author}{t.archived ? " · archived (a new message opens it again)" : ""}</p>
          {#if t.quote?.author}
            <button type="button" class="thread-start" onclick={() => {
                if (phone) threadId = null; // the channel is under it
                jumpTarget = t.start;
              }}>
              <strong>{t.quote.author}</strong>
              {hideSpoilers(t.quote.text)}
            </button>
          {:else}
            <p><em>The original message was deleted.</em></p>
          {/if}
        </div>
      {/snippet}
      {#snippet tools()}
        <button type="button" title="Thread actions" aria-label="Thread actions" onclick={(e) => openThreadMenu(t.id, e)}>
          <span aria-hidden="true">⋯</span>
        </button>
      {/snippet}
    </ChatPane>
  {/snippet}
  <div class="main" inert={phone && panel !== "chat"}>
    {#if phone && thread}
      {@render threadPane(thread)}
    {:else}
    <ChatPane
      {...paneCommon}
      {...pane}
      {...chatCommon}
      {...(readOnly ? { onSend: undefined, onUpload: undefined } : {})}
      chatKey={chatKeyOf(view, channelId)}
      {slow}
      onSearch={text.available
        ? (q) => {
            searchQuery = q;
            connection.search(parseSearch(q, searchChats));
            openSide("search");
          }
        : undefined}
      stage={streamClient?.watching.length ? stage : undefined}
      error={("error" in pane ? pane.error : null) ?? streamClient?.error}
      onPins={text.voiceChats && chatId !== null
        ? () => openSide(toggleSide(side, !phone || panel === "members", "pins"))
        : undefined}
      pinsOpen={side === "pins"}
      mayPin={directChatId !== null || canModerate(server, text, viewMumble)}
      jumpTarget={chatId !== null && text.loaded[chatId] ? jumpTarget : null}
      onJumped={() => (jumpTarget = null)}
      {searchPlaces}
      linkTo={chatId !== null && voiceChatId === null ? (id) => messageLink(site, chatId!, id) : undefined}
      typing={typingText(typingHere)}
      {onTyping}
      threadOf={textId === null ? undefined : (id) => threadByStart.get(id)}
      onOpenThread={openThread}
      onCreateThread={textId === null || readOnly ? undefined : (id, body) => (naming = { create: id, name: defaultThreadName(body) })}
      onThreads={textId === null ? undefined : () => openSide(toggleSide(side, !threadAside && (!phone || panel === "members"), "threads"))}
      threadsOpen={side === "threads" && !threadAside}
    />
    {/if}
  </div>
  {#if threadAside && thread}
    <aside class="thread-pane">{@render threadPane(thread)}</aside>
  {:else if side === "threads" && textId !== null}
    {@const list = text.archived[textId]}
    <ThreadList
      inert={phone && panel !== "members"}
      channel={textChannel?.name ?? ""}
      threads={threadList(text.threads, list?.threads ?? [], textId)}
      more={!list || archivedAsked !== null ? null : list.more}
      unread={unreadText}
      {mentions}
      onOpen={(t) => openThread(t)}
      onMore={() => loadArchived(textId!)}
      onClose={() => openSide("members")}
    />
  {:else if side === "search"}
    <SearchPanel
      inert={phone && panel !== "members"}
      text={text.search ? searchQuery : null}
      messages={(text.search?.messages ?? []).filter((m) => !blockedUsers.has(m.author))}
      where={whereOf}
      canOpen={(m) => text.channels.some((c) => c.id === m.channel || c.id === text.threads[m.channel]?.channel) || m.channel in text.directs}
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
      roleOf={(u) => badgeOf(server, text.roles, viewMumble, u.userId)}
      {statusOf}
      {offline}
      {hidden}
      {readers}
      voiceFirst={view.kind === "voice"}
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
  .layout.thread {
    grid-template-columns: 72px 240px 1fr minmax(320px, 420px);
  }
  .thread-pane {
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-left: 1px solid var(--border);
  }
  .thread-pane > :global(main) {
    flex: 1;
  }
  .thread-intro h2 {
    margin: 0 0 4px;
    font-size: 20px;
  }
  .thread-intro p {
    margin: 0 0 8px;
    color: var(--text-muted);
  }
  .thread-start {
    display: block;
    width: 100%;
    padding: 8px 12px;
    border-left: 3px solid var(--border);
    border-radius: 4px;
    background: var(--bg-sidebar);
    text-align: left;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
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
