package mumbled

import (
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// Hidden channels: a user sees a channel only with Traverse in it and in
// every channel above (unlike Murmur, which sends all). Others never get
// its ChannelState, nor the users in it (or their comments). Each client
// keeps the channels it was told about (client.seen); after changes of
// access syncViewLocked sends the difference.

// Sees: the user may see the channel. Traverse is the ACLs' (Permissions
// asks Sees for channel moderators, and adds no Traverse).
func (s *State) Sees(u *User, id uint32) bool {
	chain := s.chain(id)
	if len(chain) == 0 {
		return false
	}
	for _, ch := range chain[1:] { // root is always seen
		if s.aclPermissions(u, ch.ID)&Traverse == 0 {
			return false
		}
	}
	return true
}

// Visible: the channels the user sees.
func (s *State) Visible(u *User) map[uint32]bool {
	out := map[uint32]bool{}
	for _, ch := range s.Tree() {
		if ch.ID == 0 || out[ch.Parent] && s.aclPermissions(u, ch.ID)&Traverse != 0 {
			out[ch.ID] = true
		}
	}
	return out
}

// visibleAncestor is the user's channel, or the nearest one above it that
// the user sees.
func (s *State) visibleAncestor(u *User) uint32 {
	id := u.Channel
	for id != 0 && !s.Sees(u, id) {
		id = s.Channels[id].Parent
	}
	return id
}

// forViewer leaves out listened channels the viewer does not see (a copy
// if any are left out).
func forViewer(m *pb.UserState, seen map[uint32]bool) *pb.UserState {
	hidden := func(id uint32) bool { return !seen[id] }
	if !slices.ContainsFunc(m.ListeningChannelAdd, hidden) && !slices.ContainsFunc(m.ListeningChannelRemove, hidden) {
		return m
	}
	out := proto.CloneOf(m)
	out.ListeningChannelAdd = slices.DeleteFunc(out.ListeningChannelAdd, hidden)
	out.ListeningChannelRemove = slices.DeleteFunc(out.ListeningChannelRemove, hidden)
	return out
}

// sendUserLocked sends a change of a user to the clients that see it.
func (s *Server) sendUserLocked(u *User, m *pb.UserState) {
	frame := mumble.MessageFrame(mumble.TypeUserState, m)
	for _, c := range s.clients {
		if c.sees(u) {
			c.sendUser(m, frame)
		}
	}
}

// sendUser sends a user's state (frame: m encoded, or nil) without what
// the client does not see.
func (c *client) sendUser(m *pb.UserState, frame []byte) {
	if v := forViewer(m, c.seen); v != m || frame == nil {
		frame = mumble.MessageFrame(mumble.TypeUserState, v)
	}
	c.send(frame, false)
}

// sendMoveLocked tells everyone about a user that moved from a channel:
// those who see both get m, those who saw only the old one lose the
// user, those who see only the new one get all of it. The user itself
// gets m (also to or from TextOnly).
func (s *Server) sendMoveLocked(u *User, from uint32, m *pb.UserState) {
	frame := mumble.MessageFrame(mumble.TypeUserState, m)
	var full *pb.UserState
	for _, c := range s.clients {
		self := c.session == u.Session
		before, after := self || c.seen[from], self || c.seen[u.Channel]
		switch {
		case before && after:
			c.sendUser(m, frame)
		case before:
			c.sendMsg(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(u.Session)})
		case after:
			if full == nil {
				full = userStateMsg(u)
			}
			c.sendUser(full, nil)
		}
	}
}

// showChannelLocked sends a channel's state to those who see it.
func (s *Server) showChannelLocked(ch *Channel) {
	for _, u := range s.state.Users {
		if c := s.clients[u.Session]; c != nil && (c.seen[ch.ID] || s.state.Sees(u, ch.ID)) {
			c.seen[ch.ID] = true
			c.sendMsg(mumble.TypeChannelState, s.state.channelStateFor(ch, u))
		}
	}
}

// sendChannelLocked sends a channel change to those who see the channel,
// with links only to channels they see.
func (s *Server) sendChannelLocked(m *pb.ChannelState) {
	for _, c := range s.clients {
		if !c.seen[m.GetChannelId()] {
			continue
		}
		out := m
		hidden := func(id uint32) bool { return !c.seen[id] }
		if slices.ContainsFunc(m.LinksAdd, hidden) || slices.ContainsFunc(m.LinksRemove, hidden) {
			out = proto.CloneOf(m)
			out.LinksAdd = slices.DeleteFunc(out.LinksAdd, hidden)
			out.LinksRemove = slices.DeleteFunc(out.LinksRemove, hidden)
		}
		c.sendMsg(mumble.TypeChannelState, out)
	}
}

// syncViewsLocked runs after access changed: users in channels they may
// no longer see go up, and every client gets what it sees now.
func (s *Server) syncViewsLocked() {
	users := slices.Sorted(maps.Keys(s.state.Users))
	for _, session := range users {
		u := s.state.Users[session]
		if u.textOnly() {
			continue
		}
		if to := s.state.visibleAncestor(u); to != u.Channel {
			s.moveLocked(u, to, 0)
		}
	}
	for _, session := range users {
		if u := s.state.Users[session]; u != nil {
			s.syncViewLocked(u)
		}
	}
}

// syncViewLocked sends a client the difference between the channels it
// was told about and those it sees now: hidden ones go (their users
// first, children first), new ones come (parents first, then links and
// their users).
func (s *Server) syncViewLocked(u *User) {
	c := s.clients[u.Session]
	if c == nil {
		return
	}
	now := s.state.Visible(u)
	tree := s.state.Tree()
	gone, shown := map[uint32]bool{}, map[uint32]bool{}
	var showOrder []*Channel
	for _, ch := range tree {
		if now[ch.ID] && !c.seen[ch.ID] {
			shown[ch.ID] = true
			showOrder = append(showOrder, ch)
		}
		if c.seen[ch.ID] && !now[ch.ID] {
			gone[ch.ID] = true
		}
	}
	if len(gone) == 0 && len(shown) == 0 {
		return
	}
	sessions := slices.Sorted(maps.Keys(s.state.Users))
	for _, session := range sessions {
		if gone[s.state.Users[session].Channel] {
			c.sendMsg(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(session)})
		}
	}
	for _, su := range s.streams {
		if !c.web && gone[su.channel] {
			c.sendMsg(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(su.session)})
		}
	}
	for _, ch := range slices.Backward(tree) {
		if gone[ch.ID] {
			c.sendMsg(mumble.TypeChannelRemove, &pb.ChannelRemove{ChannelId: proto.Uint32(ch.ID)})
		}
	}
	c.seen = now
	for _, ch := range showOrder {
		c.sendMsg(mumble.TypeChannelState, s.state.channelStateFor(ch, u))
	}
	for _, ch := range showOrder {
		if links := seenLinks(ch, now); len(links) > 0 {
			c.sendMsg(mumble.TypeChannelState, &pb.ChannelState{ChannelId: proto.Uint32(ch.ID), Links: links})
		}
	}
	for _, session := range sessions {
		o := s.state.Users[session]
		if shown[o.Channel] {
			c.sendUser(userStateMsg(o), nil)
		} else if add := listenedIn(o, shown); now[o.Channel] && len(add) > 0 {
			c.sendMsg(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(session), ListeningChannelAdd: add})
		}
	}
	for _, su := range s.streams {
		if !c.web && shown[su.channel] {
			c.send(su.state(), false)
		}
	}
}

// seenLinks are a channel's links to channels in seen.
func seenLinks(ch *Channel, seen map[uint32]bool) []uint32 {
	var out []uint32
	for _, l := range slices.Sorted(maps.Keys(ch.Links)) {
		if seen[l] {
			out = append(out, l)
		}
	}
	return out
}

// listenedIn are the channels of a set the user listens to.
func listenedIn(u *User, set map[uint32]bool) []uint32 {
	var out []uint32
	for _, id := range slices.Sorted(maps.Keys(u.Listening)) {
		if set[id] {
			out = append(out, id)
		}
	}
	return out
}
