# Feedback on save and apply actions

## Summary

Report: Save in Server settings → Message rules gives no feedback. Check
every place that saves or applies something (server settings tabs:
message rules, guests read-only, browser check, thread archive time, bans,
emoji, registered users; channel/category dialogs; members dialog; invite
links; slow mode; status; user settings; OBS key; channel moderators) and
make them consistent: a busy state while the server works (button
disabled with "Saving…"), a short "Saved" confirmation when the server
confirms (role=status), the server's error shown at the control, and
controls that apply at once switch back when the server refuses.

## Acceptance Criteria

- A small shared helper/pattern for pending → saved/error (tests).
- Every listed control audited; fixed where feedback is missing.
- Manual check of the main ones in a headless browser.
