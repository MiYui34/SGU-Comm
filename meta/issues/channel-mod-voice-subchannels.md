# Channel moderators create voice subchannels

## Summary

Channel moderators (see channel-moderators.md) may create, rename,
reorder and delete permanent voice channels below their voice channel, as
Mumble's tree allows (MakeChannel in their subtree). Their moderator
rights cover the new subchannels (ACL inheritance). They cannot change the
channel they moderate itself beyond what they may today, nor anything
outside their subtree.

## Acceptance Criteria

- Server tests: create/rename/delete/reorder inside the subtree; refused
  outside it and for the moderated channel itself (delete/rename/move);
  private subchannels follow the private-channel rules.
- Web: "Create subchannel" (and the edit/delete items) in the channel menu
  for channel moderators; keyboard accessible.
