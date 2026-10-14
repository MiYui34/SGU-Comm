package mumbled

import (
	"net"
	"net/netip"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
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
