package mumbled

import (
	"fmt"
	"maps"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

// expectEvent waits for the next OnUserChanged call ("id name").
func expectEvent(t *testing.T, events <-chan string, want string) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("event %q, want %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("no event, want %q", want)
	}
}

func TestUsersChangesAreReported(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	events := make(chan string, 10)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnUserChanged: func(id uint32, name string) {
		events <- fmt.Sprint(id, " ", name)
	}}, nil)
	su := superUser(t, addr)
	// A Mumble client registers itself, a web account on its first visit.
	_, aliceID := registered(t, addr, "alice")
	expectEvent(t, events, fmt.Sprint(aliceID, " alice"))
	joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "Bob", CertHash: "b0b"}}, "Bob"))
	bob, _ := db.UserByHash("b0b")
	expectEvent(t, events, fmt.Sprint(bob.ID, " Bob"))
	if users := s.Users(); len(users) != 2 || users[aliceID] != "alice" || users[bob.ID] != "Bob" {
		t.Fatalf("users %v", users) // SuperUser is not listed
	}
	// An admin renames and removes registrations.
	su.send(mumble.TypeUserList, &pb.UserList{Users: []*pb.UserList_User{{UserId: proto.Uint32(aliceID), Name: proto.String("Alicia")}}})
	expectEvent(t, events, fmt.Sprint(aliceID, " Alicia"))
	su.send(mumble.TypeUserList, &pb.UserList{Users: []*pb.UserList_User{{UserId: proto.Uint32(aliceID)}}})
	expectEvent(t, events, fmt.Sprint(aliceID, " "))
	if users := s.Users(); len(users) != 1 {
		t.Fatalf("after removal: %v", users)
	}
}

// New registrations are reported once the user is online as registered:
// a client never lists a new user as offline for a moment.
func TestNewUsersAreOnlineWhenReported(t *testing.T) {
	var s *Server
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	events := make(chan string, 10)
	_, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnUserChanged: func(id uint32, name string) {
		_, on := s.Online()[id]
		events <- fmt.Sprint(id, " ", name, " ", on)
	}}, func(srv *Server) { s = srv })
	alice, aliceID := registered(t, addr, "alice")
	expectEvent(t, events, fmt.Sprint(aliceID, " alice true"))
	bob := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "Bob", CertHash: "b0b"}}, "Bob"))
	bobReg, _ := s.cfg.DB.UserByHash("b0b")
	expectEvent(t, events, fmt.Sprint(bobReg.ID, " Bob true"))
	dial(t, addr, "guest") // guests are not listed
	superUser(t, addr)     // nor SuperUser
	want := map[uint32]OnlineUser{aliceID: {"alice", alice.session}, bobReg.ID: {"Bob", bob.session}}
	if got := s.Online(); !maps.Equal(got, want) {
		t.Fatalf("online %v, want %v", got, want)
	}
}

func TestUnregisterIsReported(t *testing.T) {
	events := make(chan string, 10)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: testDB(t), OnUserChanged: func(id uint32, name string) {
		events <- fmt.Sprint(id, " ", name)
	}}, nil)
	cert := clientCert(t)
	c := dialAs(t, addr, dialOpts{name: "visitor", cert: cert})
	c.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(c.session), UserId: proto.Uint32(0)})
	id := waitForUserID(t, c, c.session)
	expectEvent(t, events, fmt.Sprint(id, " visitor"))
	c.conn.Close()
	eventually(t, func() bool { gone, err := s.Unregister(certHashOf(cert)); return gone && err == nil })
	expectEvent(t, events, fmt.Sprint(id, " "))
}

func TestBioOfAnOfflineUser(t *testing.T) {
	s, addr := rolesServer(t)
	alice, aliceID := registered(t, addr, "alice")
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), Comment: proto.String("hi there")})
	alice.waitForState(alice.session, func(m *pb.UserState) bool { return m.Comment != nil })
	alice.conn.Close()
	eventually(t, func() bool { return s.Bio(aliceID) == "hi there" })
	if b := s.Bio(9999); b != "" {
		t.Fatalf("unknown user's bio %q", b)
	}
}

func TestRolesOfOfflineUsersChange(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	id, _ := s.cfg.DB.RegisterUser("offline", "0ff")
	if err := s.SetModerator(su.session, id, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdmin(su.session, id, true); err != nil {
		t.Fatal(err)
	}
	if admins, mods := s.Roles(); len(admins) != 1 || admins[0] != id || len(mods) != 1 || mods[0] != id {
		t.Fatalf("roles %v %v", admins, mods)
	}
}
