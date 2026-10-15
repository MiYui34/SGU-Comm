// What the channel dialog does: create, edit or delete a channel.
export type ChannelKind = "text" | "voice";
export type ChannelForm =
  | { mode: "create"; kind: ChannelKind; temporaryOnly: boolean }
  | { mode: "edit"; kind: ChannelKind; name: string; description: string }
  | { mode: "delete"; kind: ChannelKind; name: string };
