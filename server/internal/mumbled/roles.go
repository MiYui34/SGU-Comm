package mumbled

import (
	"errors"
	"maps"
	"slices"
)

// Roles, on top of Mumble's groups in root: admins (SuperUser and the
// "admin" group) may do everything; moderators (the "moderator" group) all
// but changing ACLs and groups, which needs Write in root.

// moderatorACL gives moderators everything but Write (and Speak, so a
// lobby stays silent) in root and below, and Write below root.
func moderatorACL() []ACL {
	return []ACL{
		{ApplyHere: true, ApplySubs: true, Group: "moderator", Allow: AllPerms &^ (Write | Speak)},
		{ApplyHere: false, ApplySubs: true, Group: "moderator", Allow: Write},
	}
}

// Roles lists registered user ids by role (SuperUser not included).
type Roles struct {
	Admins     []uint32
	Moderators []uint32
}

func (s *State) roles() Roles {
	root := s.Channels[0]
	ids := func(group string) []uint32 { return slices.Sorted(maps.Keys(s.members(root, group))) }
	return Roles{Admins: ids("admin"), Moderators: ids("moderator")}
}

// isStaff: SuperUser, admins and moderators. Only admins act on them
// (kick, ban, rename, unregister).
func (s *State) isStaff(id uint32) bool {
	r := s.roles()
	return id == 0 || slices.Contains(r.Admins, id) || slices.Contains(r.Moderators, id)
}

func (s *Server) Roles() Roles {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.roles()
}

// SetModerator makes a registered user a moderator, or not. Only admins
// (Write in root) may.
func (s *Server) SetModerator(actor, userID uint32, on bool) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if userID == 0 || !s.registeredIDs([]uint32{userID})[userID] {
		return errors.New("not a registered user")
	}
	s.mu.Lock()
	u := s.state.Users[actor]
	if u == nil || !s.state.Has(u, 0, Write) {
		s.mu.Unlock()
		return errors.New("only admins make moderators")
	}
	root := s.state.Channels[0]
	if root.Groups == nil {
		root.Groups = map[string]*Group{}
	}
	g := root.Groups["moderator"]
	if g == nil {
		g = newGroup("moderator")
		root.Groups["moderator"] = g
	}
	if on {
		g.Add[userID] = true
		delete(g.Remove, userID)
	} else {
		delete(g.Add, userID)
	}
	s.permissionsChangedLocked(root)
	save := cloneChannel(root)
	s.mu.Unlock()
	s.save([]*Channel{save}, nil, 0)
	s.rolesChanged()
	return nil
}

// rolesChanged tells the Kallisti server (not under s.mu).
func (s *Server) rolesChanged() {
	if s.cfg.OnRolesChanged != nil {
		s.cfg.OnRolesChanged()
	}
}
