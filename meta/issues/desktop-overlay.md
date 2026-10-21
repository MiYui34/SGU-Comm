# Desktop: overlay of who is talking

## Summary

In the desktop app: a small always-on-top, click-through window (corner of
the screen) with the people in your voice channel and who is talking, so
gamers see it over a game. Works over windowed and borderless games.
A real in-game overlay for exclusive full screen needs hooking the game's
graphics (as Discord does on Windows): out of scope.

## Acceptance Criteria

- Tauri second window: transparent, always on top, ignores the mouse,
  position and size remembered; on/off in Settings and a hotkey.
- Shows avatars/names, speaking ring, mute icons; fades when nobody talks
  (option).
- Manual check on macOS and Windows over a borderless game or window.
