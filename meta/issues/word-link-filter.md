# Word and link filter

## Summary

Like a small AutoMod: admins keep a list of blocked words or patterns and
blocked link domains. A matching message is refused (the author sees
why); optionally moderators get a note. Simple matching (whole words,
case-insensitive), no regular expressions from users.

## Acceptance Criteria

- Server tests: matching rules (words, domains, Unicode case); staff
  exempt; edits are checked too.
- Web: list management in the server settings.
