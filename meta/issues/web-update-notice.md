# Web: notice when a new version is deployed

## Summary

After a deploy, open tabs keep running the old web app until someone reloads.

## Requirements

- The app compares its own main script (content-hashed name) with the one in the server's current index.html: after reconnects, when the tab becomes visible, and every 15 minutes.
- A newer version shows a banner with "Reload" (no automatic reload: it would end voice) and a close button.
- Not in the desktop app (it carries its own frontend) or in development.

## Acceptance Criteria

- Unit tests for the comparison; checked in the browser with a rebuilt app.
