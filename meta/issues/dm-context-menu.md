# Direct messages: context menu

## Summary

Report: no way to manage direct messages in the sidebar. Like Discord,
right-click (long press on phones) a conversation: Close (hides it until a
new message comes), Mark as read, Mute (as for channels), Profile, Pin to
top (our extra; pinned ones first). No real delete: the other person keeps
the conversation.

## Acceptance Criteria

- Pure functions for the DM list (closed, pinned, order, a new message
  reopens) with tests; state saved per account (server-side if DM state
  lives there, else on the device — decide, document).
- Keyboard accessible (menu key / Shift+F10), as the channel menu.
