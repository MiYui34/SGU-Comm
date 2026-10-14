package mumbled

import (
	"slices"
	"strconv"
	"strings"
)

// Perm is Mumble's permission bit set (ChanACL::Perm).
type Perm uint32

const (
	Write           Perm = 0x1
	Traverse        Perm = 0x2
	Enter           Perm = 0x4
	Speak           Perm = 0x8
	MuteDeafen      Perm = 0x10
	Move            Perm = 0x20
	MakeChannel     Perm = 0x40
	LinkChannel     Perm = 0x80
	Whisper         Perm = 0x100
	TextMessage     Perm = 0x200
	MakeTempChannel Perm = 0x400
	Listen          Perm = 0x800
	// Granted in the root channel only, as they act on the whole server.
	Kick             Perm = 0x10000
	Ban              Perm = 0x20000
	Register         Perm = 0x40000
	SelfRegister     Perm = 0x80000
	ResetUserContent Perm = 0x100000

	rootOnly    = Kick | Ban | Register | SelfRegister | ResetUserContent
	AllPerms    = Write | Traverse | Enter | Speak | MuteDeafen | Move | MakeChannel | LinkChannel | Whisper | TextMessage | MakeTempChannel | Listen | rootOnly
	defaultPerm = Traverse | Enter | Speak | Whisper | TextMessage | Listen
)

// ACL is one entry of a channel's access list, for a user or a group.
type ACL struct {
	ApplyHere, ApplySubs bool
	HasUser              bool
	UserID               uint32 // if HasUser
	Group                string // otherwise
	Allow, Deny          Perm
}

// Group is a channel group. Add and Remove hold registered user ids;
// Remove takes away members inherited from groups of the same name above.
type Group struct {
	Name                 string
	Inherit, Inheritable bool
	Add, Remove          map[uint32]bool
}

func newGroup(name string) *Group {
	return &Group{Name: name, Inherit: true, Inheritable: true, Add: map[uint32]bool{}, Remove: map[uint32]bool{}}
}

// DefaultRootACL is Murmur's ACL for a new server: admins may do
// everything, registered users may make temporary channels, and everyone
// may register.
func DefaultRootACL() ([]ACL, map[string]*Group) {
	return []ACL{
		{ApplyHere: true, ApplySubs: true, Group: "admin", Allow: Write},
		{ApplyHere: true, ApplySubs: true, Group: "auth", Allow: MakeTempChannel},
		{ApplyHere: true, ApplySubs: false, Group: "all", Allow: SelfRegister},
	}, map[string]*Group{"admin": newGroup("admin")}
}

// chain lists the channels from root down to id.
func (s *State) chain(id uint32) []*Channel {
	var out []*Channel
	for c := s.Channels[id]; c != nil; c = s.Channels[c.Parent] {
		out = append(out, c)
		if c.ID == 0 {
			break
		}
	}
	slices.Reverse(out)
	return out
}

func isSuperUser(u *User) bool { return u.Registered && u.UserID == 0 }

// Permissions is what a user may do in a channel, as Murmur computes it
// (ChanACL::effectivePermissions).
func (s *State) Permissions(u *User, channel uint32) Perm {
	if isSuperUser(u) {
		return AllPerms &^ (Speak | Whisper)
	}
	target := s.Channels[channel]
	if target == nil {
		return 0
	}
	granted := defaultPerm
	traverse, write := true, false
	for _, ch := range s.chain(channel) {
		if ch.NoInheritACL {
			granted = defaultPerm
		}
		for _, acl := range ch.ACLs {
			matchUser := acl.HasUser && u.Registered && acl.UserID == u.UserID
			matchGroup := !acl.HasUser && s.inGroup(target, ch, acl.Group, u)
			if !matchUser && !matchGroup {
				continue
			}
			here := ch == target && acl.ApplyHere
			inherited := ch != target && acl.ApplySubs
			apply := here || inherited
			// Traverse denied anywhere above blocks the way, even if that
			// ACL is not handed down.
			if inherited || acl.ApplyHere {
				if acl.Allow&Traverse != 0 {
					traverse = true
				}
				if acl.Deny&Traverse != 0 {
					traverse = false
				}
			}
			if apply {
				if acl.Allow&Write != 0 {
					write = true
				}
				if acl.Deny&Write != 0 {
					write = false
				}
			}
			if ch.ID == 0 && here {
				granted |= acl.Allow & rootOnly
			}
			if apply {
				granted |= acl.Allow &^ rootOnly
				granted &^= acl.Deny
			}
		}
		if !traverse && !write {
			return 0
		}
	}
	if granted&Write != 0 {
		granted |= Traverse | Enter | MuteDeafen | Move | MakeChannel | LinkChannel | TextMessage | MakeTempChannel | Listen
		if channel == 0 {
			granted |= rootOnly
		}
	}
	return granted
}

// Has reports whether the user has all of the permissions in a channel.
func (s *State) Has(u *User, channel uint32, p Perm) bool {
	return s.Permissions(u, channel)&p == p
}

// inGroup is Group::appliesToUser: target is the channel the permission is
// for, aclChannel the channel that holds the ACL entry.
func (s *State) inGroup(target, aclChannel *Channel, spec string, u *User) bool {
	invert, token, hash := false, false, false
	context := target
loop:
	for spec != "" {
		switch spec[0] {
		case '!':
			invert = true
		case '~':
			context = aclChannel
		case '#':
			token = true
		case '$':
			hash = true
		default:
			break loop
		}
		spec = spec[1:]
	}
	if spec == "" {
		return false
	}
	var match bool
	switch {
	case token:
		match = slices.Contains(u.Tokens, spec)
	case hash:
		match = u.Hash != "" && u.Hash == spec
	case spec == "none", spec == "strong": // no verified certificates
		match = false
	case spec == "all":
		match = true
	case spec == "auth":
		match = u.Registered
	case spec == "in":
		match = u.Channel == context.ID
	case spec == "out":
		match = u.Channel != context.ID
	case spec == "sub" || strings.HasPrefix(spec, "sub,"):
		match = s.inSub(target, context, strings.TrimPrefix(strings.TrimPrefix(spec, "sub"), ","), u)
	default:
		match = s.groupMember(context, spec, u)
	}
	return match != invert
}

// inSub is @sub[,offset[,min[,max]]]: the user is in a channel below the
// context channel (moved by offset), between min and max levels deeper.
func (s *State) inSub(target, context *Channel, args string, u *User) bool {
	offset, minLevel, maxLevel := 0, 1, 1000
	for i, a := range strings.Split(args, ",") {
		n, err := strconv.Atoi(a)
		if a == "" || err != nil {
			continue
		}
		switch i {
		case 0:
			offset = n
		case 1:
			minLevel = n
		case 2:
			maxLevel = n
		}
	}
	home := s.chain(u.Channel)
	current := s.chain(target.ID)
	idx := slices.Index(current, context) + offset
	if idx >= len(current) {
		return false
	}
	idx = max(idx, 0)
	if !slices.Contains(home, current[idx]) {
		return false
	}
	depth := len(home) - 1
	return depth >= idx+minLevel && depth <= idx+maxLevel
}

// groupMember follows a named group from the context channel up through
// inheriting groups, then applies adds and removes from the top down.
func (s *State) groupMember(context *Channel, name string, u *User) bool {
	if !u.Registered {
		return false
	}
	var stack []*Group
	for _, ch := range slices.Backward(s.chain(context.ID)) {
		g := ch.Groups[name]
		if g == nil {
			continue
		}
		if ch != context && !g.Inheritable {
			break
		}
		stack = append(stack, g)
		if !g.Inherit {
			break
		}
	}
	member := false
	for _, g := range slices.Backward(stack) {
		if g.Add[u.UserID] {
			member = true
		}
		if g.Remove[u.UserID] {
			member = false
		}
	}
	return member
}
