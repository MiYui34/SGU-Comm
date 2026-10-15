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

## Result

- Admins create, rename, reorder and delete text channels (sidecar, Write
  in root). Voice channels use Mumble's ACLs: "+" in the category header
  and a right-click menu (Edit, Move up/down, Delete). Users with only
  MakeTempChannel get temporary channels.
- Tested in Chrome as SuperUser and as a guest. Not done: phones have no
  right-click, so the menu is desktop-only for now.
