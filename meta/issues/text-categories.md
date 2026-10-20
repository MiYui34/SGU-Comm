# Text channel categories

## Summary

Like Discord: text channels can be grouped in categories — one level,
collapsible headings in the sidebar ("Gaming": #minecraft, #valorant).
Staff create, rename, reorder and delete categories and move channels
between them. A category has channel moderators like a channel; they may
create, rename, reorder and delete text channels inside it (the Discord
"Manage Channels on a category" equivalent) and have their moderator
rights there. Channels without a category stay at the top.

## Acceptance Criteria

- Design: how categories map to the server (store column/table for text
  channels; the mirror tree for Mumble clients — e.g. a Mumble channel per
  category holding its text channels' mirrors, so ACL inheritance gives
  category moderators their rights).
- Server tests: category CRUD (staff), moving channels, category
  moderators create/manage channels only inside their category, rights
  inherit; private channels inside categories.
- Web: collapsible categories (state per device), drag or menu to move
  channels, quick switcher shows the category.
