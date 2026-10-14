# Mumble server: whispers and shouts

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- VoiceTarget (users, channels, linked channels, children, groups);
  audio with a target goes only to those users, with the right context.
- Channel listeners (Mumble 1.4): users hear a channel they are not in
  (listening_channel_add/remove, Listen permission).

## Acceptance Criteria

- A whisper to one user is heard only by that user.
