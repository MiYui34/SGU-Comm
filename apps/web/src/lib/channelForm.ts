import type { ListenOnly, Speaker } from "./listenOnly";
import type { Member } from "./privateChannels";

// What the channel dialog does: create, edit or delete a channel.
// Private channels (built-in server) for admins and moderators: canPrivate
// on create; private on edit (with the members, which stay). A voice
// channel made in another (parent) is a subchannel.
export type ChannelKind = "text" | "voice";
export type ChannelForm =
  | { mode: "create"; kind: ChannelKind; temporaryOnly: boolean; canPrivate?: boolean; parent?: number }
  | {
      mode: "edit";
      kind: ChannelKind;
      name: string;
      description: string;
      // Voice channels with the built-in server; users: who may become a
      // speaker.
      listenOnly?: ListenOnly & { users: Speaker[] };
      private?: { on: boolean; members: Member[] };
      // Text channels with the built-in server: seconds (see lib/slowMode).
      slowMode?: number;
      // A text channel's moderator: only the topic and slow mode change.
      topicOnly?: boolean;
    }
  | { mode: "delete"; kind: ChannelKind; name: string };
