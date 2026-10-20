package mumbled

import "slices"

// Voice subchannels of channel moderators: they make permanent channels
// in the voice channels they moderate (at any depth), and rename, reorder,
// move and delete those below the moderated channel, never it itself or
// anything outside. Their moderator rights cover the new channels (groups
// inherit). Not ACL entries, but State.Permissions, so Mumble clients see
// them: Write from the ACLs would let them edit the ACL (see mayEditACL),
// and a creator with MakeChannel from the ACLs becomes the channel's admin.

// modPermissions: MakeChannel in a voice channel the user moderates, and
// Write too if it moderates the channel above as well.
func (s *State) modPermissions(u *User, channel uint32) Perm {
	ch := s.Channels[channel]
	if ch == nil || channel == 0 || s.mirror(channel) || !s.isChannelMod(u, channel) {
		return 0
	}
	if ch.Parent != 0 && s.isChannelMod(u, ch.Parent) {
		return MakeChannel | Write
	}
	return MakeChannel
}

// mirror: a text channel mirror, their parent or below them.
func (s *State) mirror(channel uint32) bool { return s.Mirror != nil && s.Mirror(channel) }

// mayMove: Write in the channel and MakeChannel in the new parent, both
// from the ACLs or both a channel moderator's: nothing leaves or enters
// the moderated subtree.
func (s *State) mayMove(u *User, channel, parent uint32) bool {
	acl := s.aclPermissions(u, channel)&Write != 0 && s.aclPermissions(u, parent)&MakeChannel != 0
	mod := s.modPermissions(u, channel)&Write != 0 && s.modPermissions(u, parent)&MakeChannel != 0
	return acl || mod
}

// mayRemove: Write in the channel; a channel moderator's only with it in
// every channel below too (a private one it does not see stays).
func (s *State) mayRemove(u *User, channel uint32) bool {
	return s.aclPermissions(u, channel)&Write != 0 ||
		!slices.ContainsFunc(s.Subtree(channel), func(id uint32) bool { return s.modPermissions(u, id)&Write == 0 })
}
