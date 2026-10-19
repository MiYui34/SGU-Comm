# Paste an image in the desktop app

## Summary

Report: pasting an image to attach it works in browsers, not in the Linux
AppImage. WebKitGTK 2.50 gives the paste event no files, items or types
for an image (checked in podman, see DEVLOG "Paste in the desktop app").
The clipboard API reads the image in the paste handler, so the desktop
app reads it there on a paste without files or text.

## Acceptance Criteria

- Pure function for the pasted files (files, else file items), tests.
- Desktop fallback: the clipboard API (no native plugin needed).
- Check by a Linux user.
