package mumbled

import (
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
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
	cfg := Config{OnChannelText: func(from uint32, name string, registered bool, channel uint32, text string) {
		mu.Lock()
		got = append(got, heardText{from, name, channel, text})
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
