# Account UX follow-ups

## Summary

Small items from the account reviews.

## Requirements

- Show a notice when Murmur refuses the self-registration (PermissionDenied); today it fails silently.
- After "This name belongs to an account", open the Log in tab with the name filled in.
- If logout fails, tell the user (the cookie may still be set).
- "Log out everywhere"; a password change should end other sessions.
- With ALLOWED_ORIGINS=*, refuse credentials unless a development flag is set.

## Acceptance Criteria

- Each item above is handled or explicitly dropped.

## Result

- All items done. The PermissionDenied toast is covered by reducer tests; no UI path in the test setup triggers a refusal, so it was not seen in a browser.
