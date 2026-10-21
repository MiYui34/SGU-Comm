# A refresh keeps the open channel and voice

## Summary

After a page refresh the client should come back where it was: the open text channel, DM or thread, and the voice channel if it was in voice (as a reconnect does, and as Discord does after a reload).

## Requirements

- The place (view, open thread, voice channel, our session id) is saved per tab in sessionStorage (a refresh keeps it; a new tab starts fresh, so two tabs do not fight over voice) and used like the in-memory place of a reconnect.
- Voice rejoin after a refresh follows the reconnect rules (rejoinTarget): not when another device or tab of the account holds voice now; the restored self-mute/deafen is applied before joining.
- Browsers may start the AudioContext suspended without a user gesture: resume it on the first pointer or key input, and show a small hint ("Click anywhere to turn on sound") while it is suspended in voice.
- Places that no longer exist (deleted channel, no access) fall back as today.

## Acceptance Criteria

- Unit tests for the save/load logic; web tests and svelte-check pass; headless check: open a text channel and a thread, refresh, same view; in voice, refresh, back in voice (muted if muted).
