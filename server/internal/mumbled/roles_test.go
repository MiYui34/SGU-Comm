package mumbled

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

// registered joins with a certificate and registers itself.
func registered(t *testing.T, addr, name string) (*testClient, uint32) {
	t.Helper()
	c := dialAs(t, addr, dialOpts{name: name, cert: clientCert(t)})
	c.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(c.session), UserId: proto.Uint32(0)})
	return c, waitForUserID(t, c, c.session)
}

func rolesServer(t *testing.T) (*Server, string) {
	t.Helper()
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	return startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
}

func TestModeratorsDoAlmostAllThatAdminsDo(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	alice, aliceID := registered(t, addr, "alice")
	if err := s.SetModerator(su.session, aliceID, true); err != nil {
		t.Fatal(err)
	}
	// Channels: make, rename, delete.
	games := alice.createChannel(0, "Games", false)
	alice.send(mumble.TypeChannelState, &pb.ChannelState{ChannelId: proto.Uint32(games), Name: proto.String("Fun")})
	for {
		var m pb.ChannelState
		proto.Unmarshal(alice.expect(mumble.TypeChannelState), &m)
		if m.GetChannelId() == games && m.GetName() == "Fun" {
			break
		}
	}
	// Kick a guest.
	guest := dial(t, addr, "guest")
	alice.send(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(guest.session), Reason: proto.String("bye")})
	guest.expect(mumble.TypeUserRemove)
}

// Each test client sends at most 5 messages at once (the server's limit).
func TestModeratorsCannotHandOutRightsOrActOnStaff(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	games := su.createChannel(0, "Games", false)
	alice, aliceID := registered(t, addr, "alice")
	s.SetModerator(su.session, aliceID, true)
	// No ACL changes, in root or below, and no new moderators.
	alice.setACL(0, true, nil, nil)
	alice.expectDenied(pb.PermissionDenied_Permission)
	alice.setACL(games, true, nil, nil)
	alice.expectDenied(pb.PermissionDenied_Permission)
	_, bobID := registered(t, addr, "bob")
	if err := s.SetModerator(alice.session, bobID, true); err == nil {
		t.Fatal("a moderator made a moderator")
	}
	// Admins and moderators are safe from moderators.
	carol, carolID := registered(t, addr, "carol")
	s.SetModerator(su.session, carolID, true)
	alice.send(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(carol.session)})
	alice.expectDenied(pb.PermissionDenied_Permission)
	alice.send(mumble.TypeUserList, &pb.UserList{Users: []*pb.UserList_User{{UserId: proto.Uint32(carolID)}}})
	alice.expectDenied(pb.PermissionDenied_Permission)
}

func TestRolesListsAndUnmakesModerators(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	_, aliceID := registered(t, addr, "alice")
	s.SetModerator(su.session, aliceID, true)
	if _, mods := s.Roles(); len(mods) != 1 || mods[0] != aliceID {
		t.Fatalf("moderators %v", mods)
	}
	s.SetModerator(su.session, aliceID, false)
	if _, mods := s.Roles(); len(mods) != 0 {
		t.Fatalf("after unmaking: %v", mods)
	}
}

func TestInitRolesAddsTheModeratorGroupOnce(t *testing.T) {
	db := testDB(t)
	// An older database: root has Murmur's ACL without moderators.
	acls, groups := DefaultRootACL()
	root := &Channel{ID: 0, Name: "Root", ACLs: acls[:0:0], Groups: map[string]*Group{"admin": groups["admin"]}}
	for _, a := range acls {
		if a.Group != "moderator" {
			root.ACLs = append(root.ACLs, a)
		}
	}
	db.SaveChannel(root)
	for range 2 {
		if err := db.InitRoles(); err != nil {
			t.Fatal(err)
		}
	}
	saved, _ := db.Channels()
	s := NewState("Root", saved)
	got := s.Channels[0]
	n := 0
	for _, a := range got.ACLs {
		if a.Group == "moderator" {
			n++
		}
	}
	if got.Groups["moderator"] == nil || n != 2 {
		t.Fatalf("root after InitRoles: %+v", got)
	}
}
