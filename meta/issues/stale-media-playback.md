# Old media plays again; stream audio piles up while audio is frozen

## Summary

A user heard part of a track from hours ago while not in voice (desktop). A chat attachment that played earlier stays the browser's media-session target, even after a reconnect re-mounts the client, so a media key or headset event can resume it. Separately, stream audio stacks without a limit while the AudioContext is not running (about 690 MB per hour) and plays as one burst on resume.

## Requirements

- An attachment audio/video element that leaves the screen (unmount, reconnect) is paused and loses its source; the module-level `playing` reference does not keep it alive.
- Stream audio is dropped while the AudioContext is not running.

## Acceptance Criteria

- Tests for both; web tests and svelte-check pass.
