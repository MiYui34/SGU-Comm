# Desktop: message and invite links use the server's address

## Summary

Message and invite links are built from the page's address. In the
desktop app that is the app's own address (tauri://localhost), so the
links do not work for others. Use the Kallisti server's address there.

## Acceptance Criteria

- In the desktop app, "Copy link to message" and new invite links start
  with the server's address; message links to that server open in the app.
- The web app does not change.
