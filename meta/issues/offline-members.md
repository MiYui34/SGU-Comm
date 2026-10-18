# Offline members in the member list

## Summary

Like Discord: the member list shows registered users who are offline, in
an "Offline — N" group under the online ones, faded. Their user menu
works as for online users where it makes sense: profile (bio), direct
message, make or remove moderator/admin, block. Offline guest accounts
are not listed (they are throwaway accounts).

## Acceptance Criteria

- Server: the client gets the registered users (id, name, role, bio,
  picture) and updates when users register, are renamed or removed; tests.
- A direct message to an offline user is stored and reaches them when
  they come online (with an unread count); test.
- Roles can be changed for offline users; test.
- Web: "Offline — N" group, faded rows, same user menu (voice-only items
  such as volume or move are hidden); tests for the list logic.
- Manual check in the browser.
