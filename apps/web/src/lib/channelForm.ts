import type { ListenOnly, Speaker } from "./listenOnly";

// What the channel dialog does: create, edit or delete a channel.
export type ChannelKind = "text" | "voice";
export type ChannelForm =
  | { mode: "create"; kind: ChannelKind; temporaryOnly: boolean }
  | {
      mode: "edit";
      kind: ChannelKind;
      name: string;
      description: string;
      // Voice channels with the built-in server; users: who may become a
      // speaker.
      listenOnly?: ListenOnly & { users: Speaker[] };
    }
  | { mode: "delete"; kind: ChannelKind; name: string };
