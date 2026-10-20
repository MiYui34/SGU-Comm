package mumbled

import (
	"slices"

	"kallisti.chat/server/internal/mumble/pb"
)

// Voice subchannels of channel moderators: they make permanent channels
// in the voice channels they moderate (at any depth), and rename, reorder,
// move and delete those below the moderated channel, never it itself or
// anything outside. Their moderator rights cover the new channels (groups
// inherit). Not ACL entries, but State.Permissions, so Mumble clients see
// them: Write from the ACLs would let them edit the ACL (see mayEditACL),
// and a creator with MakeChannel from the ACLs becomes the channel's admin.

// Limits for channels made with a moderator's MakeChannel (Murmur's
// channelcountlimit and channelnestinglimit, per moderated channel).
const (
	maxModSubchannels     = 50
	maxModSubchannelDepth = 5
)

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

// modLimit refuses a channel below parent made with a moderator's
// MakeChannel (callers check that) if the moderated channel has
// maxModSubchannels below it, or the new one would be more than
// maxModSubchannelDepth levels below it. A move (moved: the channel, 0 for
// a new one) is refused if the moved channel's deepest one would be; the
// count stays, as nothing enters the subtree (see mayMove). The moderated
// channel is the topmost one of the moderator's above parent, the one it
// may not edit (no Write there): with nested moderated channels, the
// outer one counts. Staff are not limited.
func (s *State) modLimit(u *User, parent, moved uint32) error {
	if s.isStaff(u) {
		return nil
	}
	top, depth := parent, 1
	if moved != 0 {
		depth += s.height(moved)
	}
	for s.modPermissions(u, top)&Write != 0 {
		top, depth = s.Channels[top].Parent, depth+1
	}
	switch {
	case moved == 0 && len(s.Subtree(top))-1 >= maxModSubchannels:
		return deny(pb.PermissionDenied_ChannelCountLimit, "This channel has the most subchannels it may have (%d)", maxModSubchannels)
	case depth > maxModSubchannelDepth:
		return deny(pb.PermissionDenied_NestingLimit, "Subchannels may be at most %d levels deep", maxModSubchannelDepth)
	}
	return nil
}

// height: the levels below a channel (0: none).
func (s *State) height(id uint32) (h int) {
	for _, c := range s.Channels {
		if c.Parent == id && c.ID != 0 {
			h = max(h, s.height(c.ID)+1)
		}
	}
	return h
}

// mirror: a text channel mirror, their parent or below them.
func (s *State) mirror(channel uint32) bool { return s.Mirror != nil && s.Mirror(channel) }

// mayMove: Write in the channel and MakeChannel in the new parent, both
// from the ACLs or both a channel moderator's (see modWritesAll): nothing
// leaves or enters the moderated subtree.
func (s *State) mayMove(u *User, channel, parent uint32) bool {
	acl := s.aclPermissions(u, channel)&Write != 0 && s.aclPermissions(u, parent)&MakeChannel != 0
	return acl || s.modWritesAll(u, channel) && s.modPermissions(u, parent)&MakeChannel != 0
}

// mayRemove: Write in the channel from the ACLs, or a channel moderator's
// (see modWritesAll).
func (s *State) mayRemove(u *User, channel uint32) bool {
	return s.aclPermissions(u, channel)&Write != 0 || s.modWritesAll(u, channel)
}

// modWritesAll: a channel moderator's Write in the channel and in every
// channel below (a private one it does not see stays where it is).
func (s *State) modWritesAll(u *User, channel uint32) bool {
	return !slices.ContainsFunc(s.Subtree(channel), func(id uint32) bool { return s.modPermissions(u, id)&Write == 0 })
}
