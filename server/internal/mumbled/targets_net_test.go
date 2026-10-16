package mumbled

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func (c *testClient) talk(target uint32) {
	c.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audioTo(target, 1, []byte{0xfc})))
}

func TestWhisperAndShoutOverTheNetwork(t *testing.T) {
	addr := startOpen(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	carol := dial(t, addr, "carol")
	games := alice.createChannel(0, "Games", false)
	carol.move(carol.session, games)
	alice.waitForMove(carol.session, games)
	// Whisper to bob (same channel as alice): only bob, as a whisper.
	alice.send(mumble.TypeVoiceTarget, &pb.VoiceTarget{Id: proto.Uint32(1), Targets: []*pb.VoiceTarget_Target{{Session: []uint32{bob.session}}}})
	alice.talk(1)
	if h := parseAudio(t, bob.expect(mumble.TypeUDPTunnel)); h.context != uint32(ContextWhisper) || h.sender != alice.session {
		t.Fatalf("bob heard %+v", h)
	}
	carol.expectNone(mumble.TypeUDPTunnel)
	// Shout to Games: carol, as a shout.
	alice.send(mumble.TypeVoiceTarget, &pb.VoiceTarget{Id: proto.Uint32(2), Targets: []*pb.VoiceTarget_Target{{ChannelId: proto.Uint32(games)}}})
	alice.talk(2)
	if h := parseAudio(t, carol.expect(mumble.TypeUDPTunnel)); h.context != uint32(ContextShout) {
		t.Fatalf("carol heard %+v", h)
	}
	bob.expectNone(mumble.TypeUDPTunnel)
	// Loopback: back to alice only.
	alice.talk(loopbackTarget)
	if h := parseAudio(t, alice.expect(mumble.TypeUDPTunnel)); h.sender != alice.session {
		t.Fatalf("loopback %+v", h)
	}
	bob.expectNone(mumble.TypeUDPTunnel)
}

func TestChannelListeners(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	games := su.createChannel(0, "Games", false)
	closed := su.createChannel(0, "Closed", false)
	su.setACL(closed, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Listen)}, nil)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.move(alice.session, games)
	bob.waitForMove(alice.session, games)
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), ListeningChannelAdd: []uint32{games, closed}})
	bob.expectDenied(pb.PermissionDenied_Permission) // Closed
	for {
		var m pb.UserState
		proto.Unmarshal(alice.expect(mumble.TypeUserState), &m)
		if m.GetSession() == bob.session && len(m.ListeningChannelAdd) > 0 {
			if len(m.ListeningChannelAdd) != 1 || m.ListeningChannelAdd[0] != games {
				t.Fatalf("listening %v", m.ListeningChannelAdd)
			}
			break
		}
	}
	alice.talk(0)
	if h := parseAudio(t, bob.expect(mumble.TypeUDPTunnel)); h.context != uint32(ContextListen) {
		t.Fatalf("bob heard %+v", h)
	}
	// New users see who listens where.
	c, _ := tryDial(t, addr, "carol")
	for {
		var m pb.UserState
		proto.Unmarshal(c.expect(mumble.TypeUserState), &m)
		if m.GetName() == "bob" {
			if len(m.ListeningChannelAdd) != 1 {
				t.Fatalf("bob for carol: %v", &m)
			}
			break
		}
	}
}

func TestListenersWhoLoseListenStopHearing(t *testing.T) {
	addr, _ := adminServer(t)
	su := superUser(t, addr)
	games := su.createChannel(0, "Games", false)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.move(alice.session, games)
	bob.waitForMove(alice.session, games)
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), ListeningChannelAdd: []uint32{games}})
	alice.waitForState(bob.session, func(m *pb.UserState) bool { return len(m.ListeningChannelAdd) > 0 })
	alice.talk(0)
	bob.expect(mumble.TypeUDPTunnel)
	su.setACL(games, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Listen)}, nil)
	m := alice.waitForState(bob.session, func(m *pb.UserState) bool { return len(m.ListeningChannelRemove) > 0 })
	if len(m.ListeningChannelRemove) != 1 || m.ListeningChannelRemove[0] != games {
		t.Fatalf("removed %v", m.ListeningChannelRemove)
	}
	alice.talk(0)
	bob.expectNone(mumble.TypeUDPTunnel)
}

func TestADeafenedListenerStopsHearing(t *testing.T) {
	addr := startOpen(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	games := alice.createChannel(0, "Games", false)
	alice.move(alice.session, games)
	bob.waitForMove(alice.session, games)
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), ListeningChannelAdd: []uint32{games}})
	for {
		var m pb.UserState
		proto.Unmarshal(alice.expect(mumble.TypeUserState), &m)
		if m.GetSession() == bob.session && len(m.ListeningChannelAdd) > 0 {
			break
		}
	}
	alice.talk(0)
	bob.expect(mumble.TypeUDPTunnel) // fills the cache
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), SelfDeaf: proto.Bool(true)})
	for {
		var m pb.UserState
		proto.Unmarshal(alice.expect(mumble.TypeUserState), &m)
		if m.GetSession() == bob.session && m.GetSelfDeaf() {
			break
		}
	}
	alice.talk(0)
	bob.expectNone(mumble.TypeUDPTunnel)
}

func TestAShoutToRegisteredUsersReachesNewRegistrations(t *testing.T) {
	addr, _ := adminServer(t)
	alice := dial(t, addr, "alice")
	bob := dialAs(t, addr, dialOpts{name: "bob", cert: clientCert(t)})
	alice.send(mumble.TypeVoiceTarget, &pb.VoiceTarget{Id: proto.Uint32(1), Targets: []*pb.VoiceTarget_Target{{ChannelId: proto.Uint32(0), Group: proto.String("auth")}}})
	alice.talk(1)
	bob.expectNone(mumble.TypeUDPTunnel) // a guest: not in @auth
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), UserId: proto.Uint32(0)})
	waitForUserID(t, alice, bob.session)
	alice.talk(1)
	if h := parseAudio(t, bob.expect(mumble.TypeUDPTunnel)); h.context != uint32(ContextShout) {
		t.Fatalf("bob heard %+v", h)
	}
}

func TestVoiceHasABandwidthLimit(t *testing.T) {
	addr := startOpen(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	big := make([]byte, 1000)
	big[0] = 0xfc
	sent := 200 // 200 kB at once: more than a second of the limit
	for i := range sent {
		alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audioTo(0, uint64(i), big)))
	}
	got := 0
	timeout := time.After(time.Second)
	for done := false; !done; {
		select {
		case f := <-bob.frames:
			if f.typ == mumble.TypeUDPTunnel {
				got++
			}
		case <-timeout:
			done = true
		}
	}
	if got == 0 || got >= sent {
		t.Fatalf("bob got %d of %d packets", got, sent)
	}
}

func TestPositionGoesOnlyToTheSameGame(t *testing.T) {
	addr := startOpen(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	carol := dial(t, addr, "carol")
	for _, c := range []*testClient{alice, bob} {
		c.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(c.session), PluginContext: []byte("game\\x00server1")})
	}
	alice.send(mumble.TypePing, &pb.Ping{})
	alice.expect(mumble.TypePing)
	bob.send(mumble.TypePing, &pb.Ping{})
	bob.expect(mumble.TypePing)
	alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel,
		encodeClientAudio(audioPacket{frame: 1, opus: []byte{0xfc}, positional: []float32{1, 2, 3}}, false)))
	if p, _ := decodeAudio(bob.expect(mumble.TypeUDPTunnel), false); len(p.positional) != 3 {
		t.Fatalf("bob (same game) got %+v", p)
	}
	if p, _ := decodeAudio(carol.expect(mumble.TypeUDPTunnel), false); p.positional != nil {
		t.Fatalf("carol (no game) got the position %v", p.positional)
	}
}
