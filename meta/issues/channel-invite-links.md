# Invite links for invite-only channels

## Summary

Moderators and admins make invite links for a private channel (in its
Members dialog): valid for 1 day, 7 days (default) or for ever; usable 1,
5, 25 or unlimited times. Opening the link (logged in, or after logging in
or joining as a guest) makes the user a member and opens the channel.
Staff see active links (uses, end) and can delete them. The link holds a
long random token; the server keeps only its hash. Needs
private-channels.

## Acceptance Criteria

- Server tests: create (staff only), redeem (adds to the members group),
  expiry, use limit, delete, unknown token; no leak of the channel to
  non-members before redeeming.
- Manual check: link opened logged in, and logged out (then login). (Done 2026-10-18.)
