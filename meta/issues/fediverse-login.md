# Log in with a fediverse account

## Summary

Many users come from the fediverse. Let them register, log in, and link existing accounts with a fediverse account through the Mastodon OAuth API (Mastodon, Pleroma/Akkoma, GoToSocial, Pixelfed, Friendica, ...), asking only for the profile.

## Requirements

- Flow: the user enters an instance; the server registers an app there (`POST /api/v1/apps`, cached per instance), sends the user to the instance's OAuth authorize page with scope `profile` (fallback `read:accounts` for older servers), handles the callback, reads `GET /api/v1/accounts/verify_credentials`.
- Identity: instance host + remote account id (not the name). Store the current `@user@host` for display.
- Log in: a linked identity logs in to its Kallisti account (normal web session).
- Register: an unlinked identity creates a new account in one step; the fedi username is suggested and can be changed on a clash. Same new-account rules as normal registration (captcha/message rules apply as they do today, decide sensibly).
- Link and unlink in user settings for existing accounts. An account must keep at least one way to log in (password or a linked identity).
- Accounts without a password can set one in settings (needed for native Mumble clients).
- Opt-in: show the linked handle on the profile as a verified `@user@host`.
- SSRF protection: HTTPS only, no private/loopback/link-local addresses (checked at connect time, also after redirects and DNS), timeouts, response size limits.
- Web client first.

## Acceptance Criteria

- Server tests for the OAuth flow against a fake instance (app registration, state/CSRF check, callback, verify_credentials), linking/unlinking, registration, SSRF guard.
- Client: login button, register step, settings link/unlink, opt-in handle; tests for pure logic.
- Headless browser check against a fake local instance.

## Notes

- Later: the desktop app (deep link or system browser), Misskey's MiAuth.
- Decisions: new accounts and linking both allowed; handle on the profile is opt-in; web first.
