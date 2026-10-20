package mumbled

import (
	"cmp"
	"slices"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// Several sessions per account (Kallisti, not Murmur, which kicks the
// older one): a registered user may connect up to maxSessionsPerUser
// times (browser tabs, devices). At most one session is in a channel:
// the voice session, the one others see. The others are text-only: in
// TextOnly, which is no channel. They get the channel tree, the users and
// their own UserState, but nobody else hears of them; no voice goes to or
// comes from them, and others cannot move them. A text-only session that
// joins a channel takes voice: the voice session becomes text-only (its
// own client gets a UserState in TextOnly, others see it leave and the
// other join) and its server mute goes along.
//
// A Mumble client cannot be text-only: it takes voice when it connects, a
// second one replaces it (as Murmur does), and it is kicked when a web
// session takes voice. Unregistered guests keep one session (findGhost).

const (
	// TextOnly is the channel of text-only sessions (see State.OnMove).
	TextOnly           = NoChannel - 1
	maxSessionsPerUser = 10
)

func (u *User) textOnly() bool { return u.Channel == TextOnly }

// outside is a channel for hooks: TextOnly is none.
func outside(channel uint32) uint32 {
	if channel == TextOnly {
		return NoChannel
	}
	return channel
}

// sameAccount: two sessions of one registered user.
func sameAccount(a, b *User) bool {
	return a.Registered && b.Registered && a.UserID == b.UserID && a.Session != b.Session
}

// siblingsLocked are the other sessions of u's account, oldest first.
func (s *Server) siblingsLocked(u *User) []*User {
	var out []*User
	for _, o := range s.state.Users {
		if sameAccount(u, o) {
			out = append(out, o)
		}
	}
	slices.SortFunc(out, func(a, b *User) int { return cmp.Compare(a.Session, b.Session) })
	return out
}

// voiceSiblingLocked is the account's session in a channel, if not u.
func (s *Server) voiceSiblingLocked(u *User) *User {
	for _, o := range s.siblingsLocked(u) {
		if !o.textOnly() {
			return o
		}
	}
	return nil
}

// sees: the client is told about the user (itself always).
func (c *client) sees(u *User) bool { return u.Session == c.session || c.seen[u.Channel] }

// takeVoiceLocked runs before u joins a channel: the account's voice
// session becomes text-only, or goes if it is a Mumble client. u takes
// over its server mute, deafen and priority.
func (s *Server) takeVoiceLocked(u *User) {
	v := s.voiceSiblingLocked(u)
	if v == nil {
		return
	}
	if u.Mute != v.Mute || u.Deaf != v.Deaf || u.PrioritySpeaker != v.PrioritySpeaker {
		u.Mute, u.Deaf, u.PrioritySpeaker = v.Mute, v.Deaf, v.PrioritySpeaker
		s.sendUserLocked(u, &pb.UserState{Session: proto.Uint32(u.Session), Mute: proto.Bool(u.Mute),
			Deaf: proto.Bool(u.Deaf), PrioritySpeaker: proto.Bool(u.PrioritySpeaker)})
	}
	if c := s.clients[v.Session]; c != nil && !c.web {
		s.kickLocked(v.Session, 0, "You joined voice on another device.", false)
		return
	}
	s.moveLocked(v, TextOnly, 0)
}

// promoteLocked runs after a session left: if it was its account's voice
// session, the newest text-only one goes to root, so others still see the
// user. Only where root is a lobby: elsewhere root has voice.
func (s *Server) promoteLocked(gone *User) {
	siblings := s.siblingsLocked(gone)
	if gone.textOnly() || len(siblings) == 0 || s.voiceSiblingLocked(gone) != nil {
		return
	}
	u := siblings[len(siblings)-1]
	if !s.state.Has(u, 0, Speak) && s.state.Has(u, 0, Enter) {
		s.moveLocked(u, 0, 0)
	}
}
