# Paste an image in the desktop app

## Summary

Report: pasting an image to attach it works in browsers, not in the Linux
AppImage. The paste handler reads only clipboardData.files; some engines
(WebKitGTK) give a pasted image only as clipboardData.items. Read both.
If WebKitGTK gives the page no image at all, the desktop app reads the
clipboard image natively (tauri clipboard plugin) on a paste without files
or text.

## Acceptance Criteria

- Pure function for the pasted files (files, else file items), tests.
- Desktop fallback through the clipboard plugin, if needed.
- Check by a Linux user.
