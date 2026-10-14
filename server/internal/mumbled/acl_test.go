package mumbled

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// aclState: root > A > B, and C below root. Root has Murmur's default ACL.
func aclState(t *testing.T) (*State, *Channel, *Channel, *Channel) {
	t.Helper()
	s := NewState("Root", nil)
	s.Channels[0].ACLs, s.Channels[0].Groups = DefaultRootACL()
	a := mustAdd(t, s, 0, "A", false)
	b := mustAdd(t, s, a.ID, "B", false)
	c := mustAdd(t, s, 0, "C", false)
	return s, a, b, c
}

func guest(channel uint32) *User { return &User{Session: 1, Name: "guest", Channel: channel} }

func member(id, channel uint32) *User {
	return &User{Session: 2, Name: "member", Registered: true, UserID: id, Channel: channel}
}

func has(p, want Perm) bool { return p&want == want }

func TestDefaultPermissions(t *testing.T) {
	s, a, _, _ := aclState(t)
	g := s.Permissions(guest(0), a.ID)
	if !has(g, Traverse|Enter|Speak|Whisper|TextMessage|Listen) || g&(Write|MakeChannel|MakeTempChannel|Move) != 0 {
		t.Fatalf("guest in A: %v", g)
	}
	// @all may self-register, granted in root only.
	if !has(s.Permissions(guest(0), 0), SelfRegister) {
		t.Fatal("guests cannot self-register")
	}
	// @auth may make temporary channels everywhere.
	if !has(s.Permissions(member(5, 0), a.ID), MakeTempChannel) || has(g, MakeTempChannel) {
		t.Fatal("MakeTempChannel is for registered users")
	}
}

func TestAdminsCanDoEverything(t *testing.T) {
	s, _, b, _ := aclState(t)
	s.Channels[0].Groups["admin"].Add[7] = true
	p := s.Permissions(member(7, 0), b.ID)
	if !has(p, Write|MakeChannel|Move|MuteDeafen|LinkChannel) {
		t.Fatalf("admin in B: %v", p)
	}
	if !has(s.Permissions(member(7, 0), 0), Kick|Ban|Register) {
		t.Fatal("admins cannot kick in root")
	}
	if has(s.Permissions(member(8, 0), b.ID), Write) {
		t.Fatal("not an admin, but Write")
	}
}

func TestSuperUser(t *testing.T) {
	s, a, _, _ := aclState(t)
	a.ACLs = append(a.ACLs, ACL{ApplyHere: true, ApplySubs: true, Group: "all", Deny: Enter})
	p := s.Permissions(member(0, 0), a.ID)
	if !has(p, Enter|Write|Kick) || p&(Speak|Whisper) != 0 {
		t.Fatalf("SuperUser: %v", p)
	}
}

func TestDenyAndAllowForOneUser(t *testing.T) {
	s, a, b, _ := aclState(t)
	a.ACLs = []ACL{
		{ApplyHere: true, ApplySubs: true, Group: "all", Deny: Enter},
		{ApplyHere: true, ApplySubs: true, HasUser: true, UserID: 5, Allow: Enter},
	}
	if has(s.Permissions(guest(0), b.ID), Enter) {
		t.Fatal("guest may enter B")
	}
	if !has(s.Permissions(member(5, 0), b.ID), Enter) {
		t.Fatal("user 5 may not enter B")
	}
}

func TestApplyHereAndApplySubs(t *testing.T) {
	s, a, b, _ := aclState(t)
	a.ACLs = []ACL{{ApplyHere: true, ApplySubs: false, Group: "all", Deny: Speak}}
	if has(s.Permissions(guest(0), a.ID), Speak) || !has(s.Permissions(guest(0), b.ID), Speak) {
		t.Fatal("apply here only")
	}
	a.ACLs = []ACL{{ApplyHere: false, ApplySubs: true, Group: "all", Deny: Speak}}
	if !has(s.Permissions(guest(0), a.ID), Speak) || has(s.Permissions(guest(0), b.ID), Speak) {
		t.Fatal("apply to subchannels only")
	}
}

func TestNotInheritingStartsFromTheDefaults(t *testing.T) {
	s, a, b, _ := aclState(t)
	a.ACLs = []ACL{{ApplyHere: true, ApplySubs: true, Group: "all", Deny: Speak}}
	b.NoInheritACL = true
	if !has(s.Permissions(guest(0), b.ID), Speak) {
		t.Fatal("B inherits although it should not")
	}
}

func TestTraverseDeniedAboveBlocksEverythingBelow(t *testing.T) {
	s, a, b, _ := aclState(t)
	// Denied for A itself only: still no way into B (as Murmur).
	a.ACLs = []ACL{{ApplyHere: true, ApplySubs: false, Group: "all", Deny: Traverse}}
	if s.Permissions(guest(0), b.ID) != 0 {
		t.Fatalf("guest in B: %v", s.Permissions(guest(0), b.ID))
	}
}

func TestGroups(t *testing.T) {
	s, a, b, c := aclState(t)
	a.Groups = map[string]*Group{"mods": {Name: "mods", Inherit: true, Inheritable: true, Add: map[uint32]bool{5: true, 6: true}, Remove: map[uint32]bool{}}}
	// B removes 6 from the inherited group.
	b.Groups = map[string]*Group{"mods": {Name: "mods", Inherit: true, Inheritable: true, Add: map[uint32]bool{}, Remove: map[uint32]bool{6: true}}}
	a.ACLs = []ACL{{ApplyHere: true, ApplySubs: true, Group: "mods", Allow: Move}}
	if !has(s.Permissions(member(5, 0), b.ID), Move) || has(s.Permissions(member(6, 0), b.ID), Move) {
		t.Fatal("group add/remove")
	}
	if !has(s.Permissions(member(6, 0), a.ID), Move) {
		t.Fatal("6 is a mod in A")
	}
	// Not inheritable: B does not see A's members.
	a.Groups["mods"].Inheritable = false
	if has(s.Permissions(member(5, 0), b.ID), Move) {
		t.Fatal("non-inheritable group seen below")
	}
	// Special groups.
	c.ACLs = []ACL{
		{ApplyHere: true, ApplySubs: true, Group: "in", Allow: MuteDeafen},
		{ApplyHere: true, ApplySubs: true, Group: "!auth", Deny: TextMessage},
		{ApplyHere: true, ApplySubs: true, Group: "$abc", Allow: LinkChannel},
		{ApplyHere: true, ApplySubs: true, Group: "#secret", Allow: MakeChannel},
	}
	inC := guest(c.ID)
	if !has(s.Permissions(inC, c.ID), MuteDeafen) || has(s.Permissions(guest(0), c.ID), MuteDeafen) {
		t.Fatal("@in")
	}
	if has(s.Permissions(guest(0), c.ID), TextMessage) || !has(s.Permissions(member(5, 0), c.ID), TextMessage) {
		t.Fatal("@!auth")
	}
	hashed := guest(0)
	hashed.Hash = "abc"
	if !has(s.Permissions(hashed, c.ID), LinkChannel) {
		t.Fatal("@$hash")
	}
	tokened := guest(0)
	tokened.Tokens = []string{"secret"}
	if !has(s.Permissions(tokened, c.ID), MakeChannel) {
		t.Fatal("@#token")
	}
}

func TestSubGroup(t *testing.T) {
	s, a, b, _ := aclState(t)
	// @sub on A: users in channels below A.
	a.ACLs = []ACL{{ApplyHere: true, ApplySubs: false, Group: "sub", Allow: MakeChannel}}
	if !has(s.Permissions(guest(b.ID), a.ID), MakeChannel) || has(s.Permissions(guest(a.ID), a.ID), MakeChannel) {
		t.Fatal("@sub")
	}
}

func TestACLsFromAnEditor(t *testing.T) {
	// Fields as an editor sends them: "inherited" unset (it defaults to
	// true in Mumble.proto), a user who is not registered.
	m := &pb.ACL{
		Acls: []*pb.ACL_ChanACL{
			{Group: proto.String("all"), Deny: proto.Uint32(uint32(Enter))},
			{UserId: proto.Uint32(9), Grant: proto.Uint32(uint32(Enter))},
			{UserId: proto.Uint32(5), Grant: proto.Uint32(0xffffffff)},
		},
		Groups: []*pb.ACL_ChanGroup{{Name: proto.String("mods"), Add: []uint32{5, 9}}},
	}
	acls, groups := aclsFromProto(m, map[uint32]bool{5: true})
	if len(acls) != 2 || acls[0].Deny != Enter || !acls[0].ApplyHere || !acls[0].ApplySubs {
		t.Fatalf("acls %+v", acls)
	}
	if acls[1].UserID != 5 || acls[1].Allow != AllPerms {
		t.Fatalf("user entry %+v", acls[1])
	}
	if g := groups["mods"]; g == nil || !g.Add[5] || g.Add[9] || !g.Inherit {
		t.Fatalf("groups %+v", groups)
	}
}
