# Rename to Kallisti

## Summary

The product name is Kallisti (kallisti.chat). Replace the working title "mumble-discord" in code and user-facing text. The Gitea repository and the local folder keep their names.

## Requirements

- UI: page title, login screen.
- Client name in the Mumble Version message ("Kallisti").
- Package scope `@kallisti/*`, root package name, Go module path `kallisti.chat/server`.
- README and DEVLOG.

## Acceptance Criteria

- No "mumble-discord" left outside meta/, the devlog history and the git remote.
- All tests, typechecks and the build pass.
