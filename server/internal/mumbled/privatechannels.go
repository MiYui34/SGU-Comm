package mumbled

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"strings"
)

// Private channels: only their members, admins and moderators see them
// (see visibility.go) and may enter them. Plain ACL entries and a group in
// the channel, so Mumble clients see and edit the same thing: "all" lose
// Traverse and Enter; the "members" group, "moderator" and "admin" get
// them back. Subchannels inherit it. A private text channel is its
// mirror's ACL.

const membersGroup = "members"

// privateBits are what non-members lose.
const privateBits = Traverse | Enter

// Member is a registered user in a private channel.
type Member = Speaker

// PrivateChannel is a channel only its members (and staff) see.
type PrivateChannel struct {
	Channel uint32
	Members []Member
}

func privateACL() []ACL {
	acls := []ACL{{ApplyHere: true, ApplySubs: true, Group: "all", Deny: privateBits}}
	for _, g := range []string{membersGroup, "moderator", "admin"} {
		acls = append(acls, ACL{ApplyHere: true, ApplySubs: true, Group: g, Allow: privateBits})
	}
	return acls
}

// isPrivateDeny: "all" may not traverse the channel (also by an entry
// from a Mumble client).
func isPrivateDeny(a ACL) bool {
	return !a.HasUser && a.Group == "all" && a.ApplyHere && a.Deny&Traverse != 0
}

func isPrivate(ch *Channel) bool { return slices.ContainsFunc(ch.ACLs, isPrivateDeny) }

// withoutPrivate drops the private entries and takes Traverse and Enter
// out of the denies for "all".
func withoutPrivate(acls []ACL) []ACL {
	allows := privateACL()[1:]
	var out []ACL
	for _, a := range acls {
		if slices.Contains(allows, a) {
			continue
		}
		if isPrivateDeny(a) {
			a.Deny &^= privateBits
			if a.Allow == 0 && a.Deny == 0 {
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

// makePrivate sets a channel's private entries and members (known ids).
func makePrivate(ch *Channel, members []uint32) {
	ch.ACLs = append(privateACL(), withoutPrivate(ch.ACLs)...)
	if ch.Groups == nil {
		ch.Groups = map[string]*Group{}
	}
	g := newGroup(membersGroup)
	g.Inherit = false // members of a channel above are not members here
	for _, id := range members {
		g.Add[id] = true
	}
	ch.Groups[membersGroup] = g
}

// Private lists the private channels with their members' names.
func (s *Server) Private() []PrivateChannel {
	s.mu.Lock()
	var out []PrivateChannel
	ids := map[uint32]bool{}
	for _, id := range slices.Sorted(maps.Keys(s.state.Channels)) {
		ch := s.state.Channels[id]
		if id == 0 || !isPrivate(ch) {
			continue
		}
		p := PrivateChannel{Channel: id}
		if g := ch.Groups[membersGroup]; g != nil {
			for _, u := range slices.Sorted(maps.Keys(g.Add)) {
				p.Members = append(p.Members, Member{ID: u})
				ids[u] = true
			}
		}
		out = append(out, p)
	}
	s.mu.Unlock()
	names, err := s.userNames(slices.Collect(maps.Keys(ids)))
	for i := range out {
		if err == nil { // users who are gone; after an error all stay
			out[i].Members = slices.DeleteFunc(out[i].Members, func(m Member) bool { return names[m.ID] == "" })
		}
		for j := range out[i].Members {
			out[i].Members[j].Name = cmp.Or(names[out[i].Members[j].ID], "?")
		}
	}
	return out
}

// SetPrivate makes a channel private with these members (registered user
// ids), or public. Only admins and moderators may.
func (s *Server) SetPrivate(actor, channel uint32, on bool, members []uint32) error {
	if len(members) > maxGroupMembers {
		return errors.New("too many members")
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	known := s.registeredIDs(members) // DB lookups outside the lock
	s.mu.Lock()
	ch, u := s.state.Channels[channel], s.state.Users[actor]
	switch {
	case ch == nil || u == nil:
		s.mu.Unlock()
		return errors.New("no such channel")
	case !s.state.isStaff(u):
		s.mu.Unlock()
		return errors.New("only admins and moderators make channels private")
	case channel == 0 || channel == s.textParent:
		s.mu.Unlock()
		return errors.New("this channel cannot be private")
	}
	if on {
		makePrivate(ch, slices.Collect(maps.Keys(known)))
	} else {
		ch.ACLs = withoutPrivate(ch.ACLs)
		delete(ch.Groups, membersGroup)
	}
	s.permissionsChangedLocked(ch)
	var save []*Channel
	if !ch.Temporary {
		save = []*Channel{cloneChannel(ch)}
	}
	s.mu.Unlock()
	s.save(save, nil, 0)
	s.privateChanged()
	return nil
}

// CreatePrivateChannel makes a private voice channel below root, with no
// members yet: only staff see it. Only admins and moderators may.
func (s *Server) CreatePrivateChannel(actor uint32, name string) (uint32, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	u := s.state.Users[actor]
	if u == nil || !s.state.isStaff(u) {
		s.mu.Unlock()
		return 0, errors.New("only admins and moderators make private channels")
	}
	ch, err := s.state.AddChannel(0, strings.TrimSpace(name), false)
	if err != nil {
		s.mu.Unlock()
		return 0, err
	}
	makePrivate(ch, nil)
	s.state.grantCreator(ch, u)
	s.showChannelLocked(ch)
	save, lastID := cloneChannel(ch), s.state.LastChannelID()
	s.mu.Unlock()
	s.save([]*Channel{save}, nil, lastID)
	s.privateChanged()
	return ch.ID, nil
}

// Sees: the session's user may see the channel (see visibility.go).
func (s *Server) Sees(session, channel uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[session]
	return u != nil && s.state.Channels[channel] != nil && s.state.Sees(u, channel)
}

// privateChanged tells the Kallisti server (not under s.mu).
func (s *Server) privateChanged() {
	if s.cfg.OnPrivateChanged != nil {
		s.cfg.OnPrivateChanged()
	}
}
