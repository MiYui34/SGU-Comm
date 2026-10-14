# Per-user volume and user context menu

## Summary

A user context menu (right-click a user in the channel list or member list) with Discord-style actions.

## Requirements

- Volume slider per user (0-200 %), applied in playback.
- Local mute for one user (their voice is not played).
- Send a direct text message (Mumble TextMessage to the session).
- Volume and local mute are saved per user name in localStorage.

## Acceptance Criteria

- Changing the volume of one user changes only that user's playback level.
- A locally muted user shows a mute icon, and their voice is not played.
- A direct message reaches only the target user and shows in the sender's chat.
- Settings survive a reload.

## Notes

- Playback today has one shared output gain. Each speaker needs its own GainNode.
