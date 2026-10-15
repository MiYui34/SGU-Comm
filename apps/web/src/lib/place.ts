// What the main pane shows. A direct message keeps the partner's name, so
// it still shows after they left.
export type View =
  | { kind: "voice" }
  | { kind: "text"; id: number }
  | { kind: "dm"; session: number; name: string };

// Where the user is, to come back to after a reconnect: the view, the
// voice channel (null: not in voice) and the self-mute state (a new session
// starts unmuted: it must not come back with a live microphone).
export type Place = { view: View; voice: number | null; selfMute: boolean; selfDeaf: boolean };
