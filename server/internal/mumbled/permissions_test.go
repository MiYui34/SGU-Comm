package mumbled

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

// adminServer has a database and a SuperUser with the password "admin".
func adminServer(t *testing.T) (string, *DB) {
	t.Helper()
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	return startWith(t, "127.0.0.1", Config{DB: db}), db
}

func superUser(t *testing.T, addr string) *testClient {
	t.Helper()
	return dialAs(t, addr, dialOpts{name: "SuperUser", password: "admin"})
}

func (c *testClient) expectDenied(want pb.PermissionDenied_DenyType) *pb.PermissionDenied {
	c.t.Helper()
	var d pb.PermissionDenied
	proto.Unmarshal(c.expect(mumble.TypePermissionDenied), &d)
	if d.GetType() != want {
		c.t.Fatalf("denied %v, want %v", &d, want)
	}
	return &d
}

func (c *testClient) setACL(channel uint32, inherit bool, acls []*pb.ACL_ChanACL, groups []*pb.ACL_ChanGroup) {
	c.send(mumble.TypeACL, &pb.ACL{ChannelId: proto.Uint32(channel), InheritAcls: proto.Bool(inherit), Acls: acls, Groups: groups})
}

func chanACL(group string, here, subs bool, allow, deny Perm) *pb.ACL_ChanACL {
	return &pb.ACL_ChanACL{Group: proto.String(group), ApplyHere: proto.Bool(here), ApplySubs: proto.Bool(subs),
		Grant: proto.Uint32(uint32(allow)), Deny: proto.Uint32(uint32(deny))}
}

func TestGuestsCannotMakeChannelsButAdminsCan(t *testing.T) {
	addr, _ := adminServer(t)
	guest := dial(t, addr, "guest")
	guest.send(mumble.TypeChannelState, &pb.ChannelState{Parent: proto.Uint32(0), Name: proto.String("Nope")})
	d := guest.expectDenied(pb.PermissionDenied_Permission)
	if Perm(d.GetPermission()) != MakeChannel || d.GetChannelId() != 0 {
		t.Fatalf("denied %v", d)
	}
	su := superUser(t, addr)
	su.createChannel(0, "Yes", false)
}

func TestRegisteredUsersMakeTemporaryChannelsAndOwnThem(t *testing.T) {
	addr, _ := adminServer(t)
	alice := dialAs(t, addr, dialOpts{name: "alice", cert: clientCert(t)})
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), UserId: proto.Uint32(0)})
	waitForUserID(t, alice, alice.session)
	tmp := alice.createChannel(0, "Mine", true)
	// The creator is admin there: it may rename it.
	alice.send(mumble.TypeChannelState, &pb.ChannelState{ChannelId: proto.Uint32(tmp), Name: proto.String("Renamed")})
	for {
		var m pb.ChannelState
		proto.Unmarshal(alice.expect(mumble.TypeChannelState), &m)
		if m.GetChannelId() == tmp && m.GetName() == "Renamed" {
			break
		}
	}
	// Guests cannot make temporary channels.
	guest := dial(t, addr, "guest")
	guest.send(mumble.TypeChannelState, &pb.ChannelState{Parent: proto.Uint32(0), Name: proto.String("Tmp"), Temporary: proto.Bool(true)})
	guest.expectDenied(pb.PermissionDenied_Permission)
}

func TestEnterCanBeDenied(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	vip := su.createChannel(0, "VIP", false)
	guest := dial(t, addr, "guest")
	su.setACL(vip, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Enter)}, nil)
	// Everyone learns that they cannot enter.
	for {
		var m pb.ChannelState
		proto.Unmarshal(guest.expect(mumble.TypeChannelState), &m)
		if m.GetChannelId() == vip && m.GetIsEnterRestricted() {
			if m.GetCanEnter() {
				t.Fatalf("channel state %v", &m)
			}
			break
		}
	}
	guest.move(guest.session, vip)
	guest.expectDenied(pb.PermissionDenied_Permission)
	// SuperUser can still move the guest in (Move beats Enter).
	su.move(guest.session, vip)
	guest.waitForMove(guest.session, vip)
}

func TestACLQueryShowsInheritedEntries(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	a := su.createChannel(0, "A", false)
	su.send(mumble.TypeACL, &pb.ACL{ChannelId: proto.Uint32(a), Query: proto.Bool(true)})
	var m pb.ACL
	proto.Unmarshal(su.expect(mumble.TypeACL), &m)
	if !m.GetInheritAcls() {
		t.Fatal("A does not inherit")
	}
	inherited, admin := 0, false
	for _, acl := range m.Acls {
		if acl.GetInherited() {
			inherited++
		}
	}
	for _, g := range m.Groups {
		admin = admin || (g.GetName() == "admin" && g.GetInherited())
	}
	// Root's admin and auth entries apply to subchannels; the SelfRegister
	// entry does not.
	if inherited != 2 || !admin {
		t.Fatalf("ACL %v", &m)
	}
	guest := dial(t, addr, "guest")
	guest.send(mumble.TypeACL, &pb.ACL{ChannelId: proto.Uint32(a), Query: proto.Bool(true)})
	guest.expectDenied(pb.PermissionDenied_Permission)
}

func TestNoSpeakMeansSuppressed(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	quiet := su.createChannel(0, "Quiet", false)
	su.setACL(quiet, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Speak)}, nil)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	bob.move(bob.session, quiet)
	alice.move(alice.session, quiet)
	for {
		var m pb.UserState
		proto.Unmarshal(bob.expect(mumble.TypeUserState), &m)
		if m.GetSession() == alice.session && m.GetSuppress() {
			break
		}
	}
	alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(1, []byte{0xfc})))
	bob.expectNone(mumble.TypeUDPTunnel)
}

func TestMovingOthersNeedsMove(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	a := su.createChannel(0, "A", false)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.move(bob.session, a)
	alice.expectDenied(pb.PermissionDenied_Permission)
	bob.expectNone(mumble.TypeUserState)
}

func TestTextNeedsPermission(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	su.setACL(0, true, []*pb.ACL_ChanACL{
		chanACL("admin", true, true, Write, 0),
		chanACL("all", true, true, 0, TextMessage),
	}, []*pb.ACL_ChanGroup{{Name: proto.String("admin"), Inherit: proto.Bool(true), Inheritable: proto.Bool(true)}})
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String("hi")})
	alice.expectDenied(pb.PermissionDenied_Permission)
	bob.expectNone(mumble.TypeTextMessage)
}

func TestTheLastChannelNeedsEnter(t *testing.T) {
	addr, db := adminServer(t)
	su := superUser(t, addr)
	vip := su.createChannel(0, "VIP", false)
	cert := clientCert(t)
	alice := dialAs(t, addr, dialOpts{name: "alice", cert: cert})
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), UserId: proto.Uint32(0)})
	id := waitForUserID(t, alice, alice.session)
	alice.conn.Close()
	su.setACL(vip, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Enter)}, nil)
	db.SetLastChannel(id, vip)
	again, _ := tryDialWith(t, addr, dialOpts{name: "alice", cert: cert})
	for {
		var m pb.UserState
		proto.Unmarshal(again.expect(mumble.TypeUserState), &m)
		if m.GetName() == "alice" {
			if m.GetChannelId() != 0 {
				t.Fatalf("alice in channel %d", m.GetChannelId())
			}
			return
		}
	}
}

func TestAdminsRegisterOthers(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	alice := dial(t, addr, "alice")
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(su.session), UserId: proto.Uint32(0)})
	alice.expectDenied(pb.PermissionDenied_Permission) // not for others
	su.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), UserId: proto.Uint32(0)})
	if waitForUserID(t, alice, alice.session) == 0 {
		t.Fatal("registered as SuperUser")
	}
}

func (c *testClient) userList() map[uint32]string {
	c.t.Helper()
	c.send(mumble.TypeUserList, &pb.UserList{})
	var m pb.UserList
	proto.Unmarshal(c.expect(mumble.TypeUserList), &m)
	out := map[uint32]string{}
	for _, u := range m.Users {
		out[u.GetUserId()] = u.GetName()
	}
	return out
}

func TestUserList(t *testing.T) {
	addr, db := adminServer(t)
	id, _ := db.RegisterUser("Bob", "bbb")
	su := superUser(t, addr)
	alice := dial(t, addr, "alice")
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), UserId: proto.Uint32(0)})
	aliceID := waitForUserID(t, alice, alice.session)
	list := su.userList()
	if list[id] != "Bob" || list[aliceID] != "alice" || list[0] != "" {
		t.Fatalf("list %v", list) // SuperUser is not listed
	}
	// Rename Bob, unregister alice (online).
	su.send(mumble.TypeUserList, &pb.UserList{Users: []*pb.UserList_User{
		{UserId: proto.Uint32(id), Name: proto.String("Robert")},
		{UserId: proto.Uint32(aliceID)},
	}})
	for {
		var m pb.UserState
		proto.Unmarshal(alice.expect(mumble.TypeUserState), &m)
		if m.GetSession() == alice.session && m.UserId != nil {
			if m.GetUserId() != 0xffffffff {
				t.Fatalf("alice: %v", &m)
			}
			break
		}
	}
	list = su.userList()
	if list[id] != "Robert" || list[aliceID] != "" {
		t.Fatalf("after changes: %v", list)
	}
	alice.send(mumble.TypeUserList, &pb.UserList{})
	alice.expectDenied(pb.PermissionDenied_Permission)
}
