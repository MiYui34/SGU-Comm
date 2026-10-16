package mumbled

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

type heardText struct {
	session uint32
	name    string
	channel uint32
	text    string
}

func TestChannelTextGoesToTheHook(t *testing.T) {
	var mu sync.Mutex
	var got []heardText
	cfg := Config{OnChannelText: func(t ChannelText) {
		mu.Lock()
		got = append(got, heardText{t.From, t.Name, t.Channel, t.Text})
		mu.Unlock()
	}}
	s, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	alice := dial(t, addr, "alice")
	games := alice.createChannel(0, "Games", false)
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{games}, Message: proto.String("<b>hello</b>")})
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{Session: []uint32{alice.session}, Message: proto.String("private")})
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 1 })
	mu.Lock()
	defer mu.Unlock()
	if got[0] != (heardText{alice.session, "alice", games, "hello"}) {
		t.Fatalf("hook got %+v", got)
	}
	_ = s
}

func TestTheSidecarAsksAndDelivers(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	s.mu.Lock()
	ch, _ := s.state.AddChannel(0, "Closed", false)
	ch.ACLs = []ACL{{ApplyHere: true, Group: "all", Deny: Enter | TextMessage}}
	closed := ch.ID
	s.mu.Unlock()
	if s.MayRead(bob.session, closed) || s.MayWrite(bob.session, closed) {
		t.Fatal("bob may use a closed channel")
	}
	if !s.MayRead(bob.session, 0) || !s.MayWrite(bob.session, 0) {
		t.Fatal("bob may not use root")
	}
	// Delivered as a Mumble text from alice to users in the channel.
	s.SendChannelText(alice.session, 0, "from the web")
	var m pb.TextMessage
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.GetActor() != alice.session || m.GetMessage() != "from the web" || len(m.ChannelId) != 1 {
		t.Fatalf("bob got %v", &m)
	}
	alice.expectNone(mumble.TypeTextMessage)
}

func TestOnlyAdminsManageTextChannels(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	guest := dial(t, addr, "guest")
	su := superUser(t, addr)
	if s.MayManage(guest.session) || !s.MayManage(su.session) {
		t.Fatal("MayManage")
	}
}

func TestChatTextIsPlainBothWays(t *testing.T) {
	var mu sync.Mutex
	var got []string
	cfg := Config{OnChannelText: func(t ChannelText) {
		channel, text := t.Channel, t.Text
		mu.Lock()
		got = append(got, fmt.Sprint(channel, ":", text))
		mu.Unlock()
	}}
	s, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	a := alice.createChannel(0, "A", false)
	b := alice.createChannel(a, "B", false)
	// Mumble clients send HTML: the hook gets plain text. A tree message
	// counts for every channel in the tree.
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{TreeId: []uint32{a}, Message: proto.String("Tom &amp; Jerry &lt;3")})
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 2 })
	mu.Lock()
	slices.Sort(got)
	if !slices.Equal(got, []string{fmt.Sprint(a, ":Tom & Jerry <3"), fmt.Sprint(b, ":Tom & Jerry <3")}) {
		t.Fatalf("hook got %v", got)
	}
	mu.Unlock()
	// Web text goes to Mumble clients escaped, not cut at "<".
	s.SendChannelText(alice.session, 0, "if a<b & c")
	var m pb.TextMessage
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.GetMessage() != "if a&lt;b &amp; c" {
		t.Fatalf("bob got %q", m.GetMessage())
	}
	// Line breaks as tags (Mumble text is HTML).
	s.SendChannelText(alice.session, 0, "one\ntwo")
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.GetMessage() != "one<br>two" {
		t.Fatalf("bob got %q", m.GetMessage())
	}
}

// Line breaks pass between clients (plainText makes them "\n"; clients
// render HTML).
func TestLineBreaksReachOtherClients(t *testing.T) {
	_, addr := startServerAndListen(t, "127.0.0.1", Config{}, openACL)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String("one<br>two<br/>three")})
	var m pb.TextMessage
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.GetMessage() != "one<br>two<br>three" {
		t.Fatalf("bob got %q", m.GetMessage())
	}
}

func TestUserChannelAndMaySpeak(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, openACL)
	c := dial(t, addr, "speaker")
	if ch, ok := s.UserChannel(c.session); !ok || ch != 0 {
		t.Fatalf("channel %d %v", ch, ok)
	}
	if _, ok := s.UserChannel(9999); ok {
		t.Fatal("a session that is not there")
	}
	if !s.MaySpeak(c.session, 0) {
		t.Fatal("may not speak with an open ACL")
	}
	s.mu.Lock()
	s.state.Users[c.session].Suppress = true
	s.mu.Unlock()
	if s.MaySpeak(c.session, 0) {
		t.Fatal("suppressed but may speak")
	}
}
