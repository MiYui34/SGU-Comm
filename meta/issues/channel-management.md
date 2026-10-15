# Channel management

## Summary

The web client cannot create, rename, move or remove channels. To be
discussed: how it works in Mumble and Discord, and what Kallisti should do.

## Notes

- Mumble: any user with MakeChannel (or MakeTempChannel) creates channels;
  Write allows editing; ACLs per channel.
- Discord: "Manage Channels" per role, categories, permission overrides.

## Decision

- Admins create, edit, reorder and delete text and voice channels;
  registered users can make temporary voice channels (as in Mumble). Roles
  later.
