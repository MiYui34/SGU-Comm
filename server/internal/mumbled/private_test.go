package mumbled

import (
	"net/netip"
	"slices"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// Private messages between registered users go to the hook (stored); web
// recipients get them from there, not twice. Mumble clients as usual.
func TestPrivateTextBetweenRegisteredUsers(t *testing.T) {
	var mu sync.Mutex
	var got []PrivateText
	db := testDB(t)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnPrivateText: func(p PrivateText) {
		mu.Lock()
		got = append(got, p)
		mu.Unlock()
	}}, openACL)
	alice, aliceID := registered(t, addr, "alice")
	bob, bobID := registered(t, addr, "bob")
	carol := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "Carol", CertHash: "c4r0l"}}, "Carol"))
	reg, _ := db.UserByHash("c4r0l")
	guest := dial(t, addr, "guest")
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{Session: []uint32{bob.session, carol.session, guest.session}, Message: proto.String("hi &amp; bye")})
	for _, c := range []*testClient{bob, guest} {
		var m pb.TextMessage
		proto.Unmarshal(c.expect(mumble.TypeTextMessage), &m)
		if m.GetMessage() != "hi &amp; bye" {
			t.Fatalf("got %q", m.GetMessage())
		}
	}
	carol.expectNone(mumble.TypeTextMessage)
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 2 })
	mu.Lock()
	defer mu.Unlock()
	to := []uint32{got[0].To, got[1].To}
	slices.Sort(to)
	want := []uint32{bobID, reg.ID}
	slices.Sort(want)
	delivered := map[uint32]bool{got[0].To: got[0].Delivered, got[1].To: got[1].Delivered}
	if !delivered[bobID] || delivered[reg.ID] {
		t.Fatalf("delivered %v (bob native, carol web)", delivered)
	}
	if !slices.Equal(to, want) || got[0].UserID != aliceID || got[0].Name != "alice" || got[0].Text != "hi & bye" {
		t.Fatalf("hook got %+v, want to %v", got, want)
	}
}

// Stored messages from the web reach the recipient's Mumble clients.
func TestSendPrivateText(t *testing.T) {
	db := testDB(t)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, openACL)
	alice, _ := registered(t, addr, "alice")
	bob, bobID := registered(t, addr, "bob")
	if !s.SendPrivateText(alice.session, "alice", bobID, "one\ntwo <3") {
		t.Fatal("not delivered")
	}
	var m pb.TextMessage
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.GetMessage() != "one<br>two &lt;3" || m.GetActor() != alice.session {
		t.Fatalf("bob got %v", &m)
	}
	if s.SendPrivateText(alice.session, "alice", 999, "nobody") {
		t.Fatal("delivered to nobody")
	}
	// From a session that is gone: named in the text.
	s.SendPrivateText(9999, "alice", bobID, "later")
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.Actor != nil || m.GetMessage() != "alice: later" {
		t.Fatalf("bob got %v", &m)
	}
}
