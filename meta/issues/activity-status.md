# Activity status: game and music

## Summary

The desktop app can show what you do, as Discord does ("Playing X",
"Listening to Y — Z"), next to your name and in your profile. Opt-in, off
by default, and easy to pause.

- Games: the desktop app compares running process names with a list of
  known games. Lists exist (Discord's public detectable-apps list; open
  community lists) — check licences before bundling; allow users to add
  their own.
- Music: no Spotify account needed. The operating system knows what plays
  in any player: Windows media session (SMTC), macOS Now Playing
  (MediaRemote), Linux MPRIS. Title and artist only.
- The web client can't see processes; it only shows others' activity.

## Acceptance Criteria

- Design first: privacy (opt-in, what is shared, with whom), the game
  list source.
- Server: an activity field next to the custom status (short text,
  kind, start time), cleared on disconnect; tests.
- Desktop: detection per platform (Rust), rate-limited updates.
