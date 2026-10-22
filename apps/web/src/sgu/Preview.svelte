<script lang="ts">
  import type { TextMessage, TextState } from "../lib/textChannels";
  import { mockChat } from "./mockChat";
  import { mockState } from "./mock";
  import Shell from "./Shell.svelte";
  import "./theme.css";

  // 预览页自己拿着这份聊天数据，表情、编辑、置顶就在这里改，
  // 不为一个静态页面去连服务器。
  type Chat = {
    available: boolean;
    channels: { id: number; name: string }[];
    messages: Record<number, TextMessage[]>;
    loaded: Record<number, boolean>;
    uploadMax?: number;
  };

  const myId = 1;
  const myName = mockState.me.name;

  let chat = $state<Chat>(mockChat as unknown as Chat);
  let next = 1000;

  function find(messageId: number): TextMessage | undefined {
    for (const list of Object.values(chat.messages)) {
      const message = list.find((row) => row.id === messageId);
      if (message !== undefined) return message;
    }
    return undefined;
  }

  function react(messageId: number, emoji: string, on: boolean) {
    const message = find(messageId);
    if (message === undefined) return;
    const rows = message.reactions ?? [];
    const found = rows.find((row) => row.emoji === emoji);
    if (on) {
      if (found === undefined) {
        message.reactions = [...rows, { emoji, users: [myName], ids: [myId] }];
      } else if (!found.ids.includes(myId)) {
        found.ids = [...found.ids, myId];
        found.users = [...found.users, myName];
      }
      return;
    }
    if (found === undefined) return;
    const ids = found.ids.filter((id) => id !== myId);
    const users = found.users.filter((user) => user !== myName);
    message.reactions = ids.length === 0 ? rows.filter((row) => row !== found) : rows.map((row) => (row === found ? { ...row, ids, users } : row));
  }

  function edit(messageId: number, body: string) {
    const message = find(messageId);
    if (message === undefined) return;
    message.text = body;
    message.edited = Date.now();
  }

  function pin(messageId: number, on: boolean) {
    const message = find(messageId);
    if (message === undefined) return;
    message.pinned = on ? Date.now() : undefined;
  }

  function send(channelId: number, body: string, extra?: { reply?: number; upload?: string }) {
    const list = chat.messages[channelId] ?? [];
    const quoted = extra?.reply === undefined ? undefined : find(extra.reply);
    const message: TextMessage = {
      id: next++,
      channel: channelId,
      author: myName,
      text: body,
      at: Date.now(),
      ...(quoted === undefined ? {} : { reply: { id: quoted.id, author: quoted.author, text: quoted.text } }),
      ...(extra?.upload === undefined ? {} : { upload: { url: "#", name: "附件", size: 0, type: "application/octet-stream" } }),
    };
    chat.messages[channelId] = [...list, message];
  }
</script>

<Shell
  snapshot={mockState}
  chat={chat as unknown as TextState}
  {myId}
  onSendText={send}
  onReactText={react}
  onEditText={edit}
  onPinText={pin}
  onUploadText={async (file) => `preview-${file.size}`}
/>
