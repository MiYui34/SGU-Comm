package mumbled

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

// webClient joins through ServeConn, as the Kallisti server's web bridge
// does: no TLS, the real IP and maybe a web account from the bridge.
func webClient(t *testing.T, s *Server, origin Origin, name string) *testClient {
	t.Helper()
	a, b := net.Pipe()
	go s.ServeConn(b, origin)
	c := startClient(t, a, 1, 5, name, "")
	t.Cleanup(func() { a.Close() })
	return c
}

func joined(t *testing.T, c *testClient) *testClient {
	t.Helper()
	var m pb.ServerSync
	proto.Unmarshal(c.expect(mumble.TypeServerSync), &m)
	c.session = m.GetSession()
	return c
}

func TestWebClientsJoinInProcess(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	native := dial(t, addr, "native")
	web := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5")}, "webby"))
	web.talk(0)
	if h := parseAudio(t, native.expect(mumble.TypeUDPTunnel)); h.sender != web.session {
		t.Fatalf("native heard %+v", h)
	}
	// Two web guests with one name from different IPs: the name is in use.
	other := webClient(t, s, Origin{Host: netip.MustParseAddr("198.51.100.7")}, "webby")
	var r pb.Reject
	proto.Unmarshal(other.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_UsernameInUse {
		t.Fatalf("reject %v", &r)
	}
}

func TestIPBansApplyToWebClients(t *testing.T) {
	s, _ := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	ip := netip.MustParseAddr("203.0.113.9")
	s.bans.Store(&[]BanEntry{{Prefix: netip.PrefixFrom(ip, 32)}})
	c := webClient(t, s, Origin{Host: ip}, "banned")
	if _, ok := <-c.frames; ok {
		// The connection may send nothing at all, or a Reject.
		var r pb.Reject
		proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	}
	joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.10")}, "fine"))
}

func TestWebAccountsAreRegistrations(t *testing.T) {
	db := testDB(t)
	reserved := map[string]bool{"alice": true}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, ReservedName: func(n string) bool { return reserved[n] }}, nil)
	account := &WebAccount{Name: "Alice", CertHash: "abc123"}
	web := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: account}, "Alice"))
	reg, _ := db.UserByHash("abc123")
	if reg == nil {
		t.Fatal("the account was not registered")
	}
	id := reg.ID
	web.conn.Close()
	again := webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: account}, "Alice")
	for {
		var m pb.UserState
		proto.Unmarshal(again.expect(mumble.TypeUserState), &m)
		if m.GetName() == "Alice" {
			if m.UserId == nil || m.GetUserId() != id {
				t.Fatalf("again: %v", &m)
			}
			break
		}
	}
	// Native clients and guests cannot use an account's name.
	native, _ := tryDial(t, addr, "alice")
	var r pb.Reject
	proto.Unmarshal(native.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_UsernameInUse && r.GetType() != pb.Reject_WrongUserPW {
		t.Fatalf("native reject %v", &r)
	}
	guest := webClient(t, s, Origin{Host: netip.MustParseAddr("198.51.100.1")}, "ALICE")
	proto.Unmarshal(guest.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_UsernameInUse && r.GetType() != pb.Reject_WrongUserPW {
		t.Fatalf("guest reject %v", &r)
	}
}

func TestReservedNamesCannotBeSelfRegistered(t *testing.T) {
	db := testDB(t)
	addr := startWith(t, "127.0.0.1", Config{DB: db, ReservedName: func(n string) bool { return n == "bob" }})
	// "bob" belongs to a web account that never used Mumble: a native
	// client cannot even join under that name.
	c, _ := tryDial(t, addr, "Bob")
	var r pb.Reject
	proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_UsernameInUse {
		t.Fatalf("reject %v", &r)
	}
}

func TestAccountNamesCannotBeRegisteredOrTaken(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	reserved := map[string]bool{"carol": true}
	cfg := Config{DB: db, ReservedName: func(n string) bool { return reserved[n] }}
	s, addr := startServerAndListen(t, "127.0.0.1", cfg, nil)
	// A guest "bob" whose name becomes an account while it is online.
	bob := dial(t, addr, "bob")
	reserved["bob"] = true
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), UserId: proto.Uint32(0)})
	bob.expectDenied(pb.PermissionDenied_UserName)
	// An admin cannot rename a registration to an account's name.
	id, _ := db.RegisterUser("alice", "aaa")
	su := superUser(t, addr)
	su.send(mumble.TypeUserList, &pb.UserList{Users: []*pb.UserList_User{{UserId: proto.Uint32(id), Name: proto.String("Carol")}}})
	su.expectDenied(pb.PermissionDenied_Text)
	// An account whose registration has another name is refused clearly.
	db.RenameUser(id, "dave")
	c := webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "alice", CertHash: "aaa"}}, "alice")
	var r pb.Reject
	proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	if r.GetReason() == "" || r.GetType() == pb.Reject_AuthenticatorFail {
		t.Fatalf("reject %v", &r)
	}
}

func TestARejectArrivesDespiteExtraBytes(t *testing.T) {
	s, _ := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	a, b := net.Pipe()
	defer a.Close()
	go s.ServeConn(b, Origin{Host: netip.MustParseAddr("203.0.113.5")})
	// One write with Version, Authenticate (an invalid name) and more.
	v1 := uint32(1<<16 | 5<<8)
	hello := append(mumble.MessageFrame(mumble.TypeVersion, &pb.Version{VersionV1: &v1}),
		mumble.MessageFrame(mumble.TypeAuthenticate, &pb.Authenticate{Username: proto.String("x")})...)
	hello = append(hello, mumble.MessageFrame(mumble.TypePing, &pb.Ping{})...)
	go a.Write(hello)
	c := startClientReader(t, a)
	var r pb.Reject
	proto.Unmarshal(c.expectWithin(mumble.TypeReject, 2*time.Second), &r)
	if r.GetType() != pb.Reject_InvalidUsername {
		t.Fatalf("reject %v", &r)
	}
}

func TestShutdownSavesTheLastChannel(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, openACL)
	account := &WebAccount{Name: "alice", CertHash: "aaa"}
	alice := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: account}, "alice"))
	games := alice.createChannel(0, "Games", false)
	alice.move(alice.session, games)
	alice.waitForMove(alice.session, games)
	s.Close()
	r, _ := db.UserByHash("aaa")
	if r == nil || r.LastChannel != games {
		t.Fatalf("after shutdown: %+v", r)
	}
}

func TestTheLobbyDoesNotRestoreTheLastChannel(t *testing.T) {
	// Kallisti: users arrive in the lobby and join voice on purpose, also
	// when they were in a voice channel last time.
	db := testDB(t)
	if err := db.InitLobby(); err != nil {
		t.Fatal(err)
	}
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	account := &WebAccount{Name: "alice", CertHash: "aaa"}
	first := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: account}, "alice"))
	var general uint32
	s.mu.Lock()
	for _, c := range s.state.Channels {
		if c.Name == "General" {
			general = c.ID
		}
	}
	s.mu.Unlock()
	first.move(first.session, general)
	first.waitForMove(first.session, general)
	first.conn.Close()
	eventually(t, func() bool { r, _ := db.UserByHash("aaa"); return r != nil && r.LastChannel == general })
	again := webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: account}, "alice")
	for {
		var m pb.UserState
		proto.Unmarshal(again.expect(mumble.TypeUserState), &m)
		if m.GetName() == "alice" {
			if m.GetChannelId() != 0 {
				t.Fatalf("back in channel %d", m.GetChannelId())
			}
			return
		}
	}
}

func eventually(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAccountRegistrationsKeepTheirName(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	reserved := map[string]bool{"alice": true}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, ReservedName: func(n string) bool { return reserved[n] }}, nil)
	joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "alice", CertHash: "aaa"}}, "alice"))
	reg, _ := db.UserByHash("aaa")
	su := superUser(t, addr)
	// Renaming it would lock the account out (its name is the account's).
	su.send(mumble.TypeUserList, &pb.UserList{Users: []*pb.UserList_User{{UserId: proto.Uint32(reg.ID), Name: proto.String("mallory")}}})
	su.expectDenied(pb.PermissionDenied_Text)
	if r, _ := db.UserByHash("aaa"); r.Name != "alice" {
		t.Fatalf("renamed to %q", r.Name)
	}
}

// A web account deleted meanwhile (a guest account cleaned up while its
// connection was on the way) is not registered again.
func TestDeletedWebAccountsAreNotRegistered(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db, WebAccountExists: func(_, hash string) bool { return hash != "gone" }}, nil)
	c := webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "ghost", CertHash: "gone"}}, "ghost")
	var r pb.Reject
	proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	if reg, _ := db.UserByHash("gone"); reg != nil || r.GetReason() == "" {
		t.Fatalf("registered %+v, reject %v", reg, &r)
	}
}
