# Fixes from the follow-up review

## Summary

The follow-up review (2026-10-20) found mostly rare timing races. Fix only the ones that happen in practice or have a real bad outcome; leave the rest.

## Requirements

- Drafts survive a reconnect (they live above the connection-keyed Client component).
- A password change and "log out everywhere" also disconnect the account's Mumble client sessions that logged in with the password.
- Combined channel save matches names as the server stores them (case-insensitive only), without accent folding.
- Native history catch-up for a text mirror checks current channel access before delivery, if this is a single check.
- Endpoints with small request bodies (login, account, WHIP offer) have a body-read deadline; uploads keep none.

## Acceptance Criteria

- Tests for each item; Go race tests, web tests and svelte-check pass.

## Notes

- Not fixed on purpose (rare, harmless or self-correcting): screen share started during a move, cross-tab push timing, resync ordering, queued moderator actions after demotion, invite id reuse, concurrent login during a password change, the Low items.
