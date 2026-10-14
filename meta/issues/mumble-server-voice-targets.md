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

## Result

- Done: VoiceTarget (users, channels with children, links and group
  filters), loopback, audio contexts, channel listeners with the Listen
  permission, a receiver cache, Murmur's voice bandwidth limit per user.
- Official client test: "Listen To Channel" reaches the server and others.
  Whispers from the official client need a configured shortcut and are only
  tested with our own test client.
