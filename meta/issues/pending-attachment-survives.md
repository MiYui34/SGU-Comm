# A picked attachment survives a reconnect and a reload

## Summary

On the Android PWA, picking an image from the device sends the app to the background; the connection drops and the client reconnects (the chat pane is rebuilt), or Android kills the page and it reloads. Both lose the picked image.

## Requirements

- The pending attachment (file, spoiler flag, caption draft) is kept per chat above the reconnect, like text drafts.
- It is also saved per chat in IndexedDB and restored after a full reload; removed when sent or discarded. Size-bounded (only files within the upload limit; old entries expire).

## Acceptance Criteria

- Tests for the store logic; web tests and svelte-check pass; headless check: pick an image, force a reconnect, the image is still pending; reload, still pending; send works.
