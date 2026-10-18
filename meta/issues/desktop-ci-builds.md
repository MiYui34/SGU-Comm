# Desktop builds on GitHub

## Summary

GitHub Actions build the desktop app for macOS (universal), Windows and
Linux: on pushes to main that touch the app (downloads on the run), and
for tags v* a draft release with the installers. Not signed yet.

## Acceptance Criteria

- A run on GitHub builds all three and offers the installers.

## Notes

- Signing (Apple Developer ID, Windows certificate) later, with secrets.
