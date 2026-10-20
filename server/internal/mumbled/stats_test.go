package mumbled

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func TestVoiceSequenceCountsGapsAndLateFrames(t *testing.T) {
	var s voiceSeq
	for _, f := range []uint64{0, 2, 4, 8} { // 20 ms frames: steps of 2; 6 is lost
		s.add(f, false)
	}
	// As the crypt: a late frame is not lost; the last one again is ignored.
	s.add(8, false)
	s.add(6, false)
	if s.good != 5 || s.lost != 0 || s.late != 1 {
		t.Fatalf("after a gap: %+v", s)
	}
	// A new transmission starts anywhere: after a terminator or a jump.
	s.add(10, true)
	s.add(500, false)
	s.add(502, false)
	s.add(90000, false)
	if s.good != 9 || s.lost != 0 || s.late != 1 {
		t.Fatalf("new transmissions: %+v", s)
	}
	// The step is the smallest one seen in a transmission.
	s = voiceSeq{}
	for _, f := range []uint64{0, 4, 6, 8, 12} {
		s.add(f, false)
	}
	if s.good != 5 || s.lost != 1 {
		t.Fatalf("step: %+v", s)
	}
}

func TestAFullQueueDropsVoiceAndCountsIt(t *testing.T) {
	c := &client{voice: make(chan []byte, 1)}
	for range 3 {
		c.send([]byte{1}, true)
	}
	if got := c.dropped.Load(); got != 2 {
		t.Fatalf("dropped %d", got)
	}
}

// Voice overtakes queued frames (avatars, history), as it would over UDP.
func TestVoiceOvertakesQueuedFrames(t *testing.T) {
	conn, peer := net.Pipe()
	defer peer.Close()
	c := &client{conn: conn, out: make(chan []byte, 4), voice: make(chan []byte, 4)}
	c.send([]byte("a"), false)
	c.send([]byte("b"), false)
	c.send([]byte("v"), true)
	go c.writer()
	got := make([]byte, 3)
	if _, err := io.ReadFull(peer, got); err != nil || string(got) != "vab" {
		t.Fatalf("got %q %v", got, err)
	}
}

// The bridge keeps a suspended page for two minutes (bridge.Options
// PingGrace); its writes to us block meanwhile.
func TestWebClientsMayStallForTheBridgeGrace(t *testing.T) {
	if got := (&client{web: true}).writeTimeout(); got <= 2*time.Minute {
		t.Errorf("web: %v", got)
	}
	if got := (&client{}).writeTimeout(); got != writeTimeout {
		t.Errorf("native: %v", got)
	}
}

func userStats(c *testClient, session uint32, statsOnly bool) *pb.UserStats {
	c.t.Helper()
	c.send(mumble.TypeUserStats, &pb.UserStats{Session: proto.Uint32(session), StatsOnly: proto.Bool(statsOnly)})
	var m pb.UserStats
	proto.Unmarshal(c.expect(mumble.TypeUserStats), &m)
	return &m
}

// sync waits until the server handled what the client sent before.
func (c *testClient) sync() {
	c.t.Helper()
	c.send(mumble.TypePing, &pb.Ping{})
	c.expect(mumble.TypePing)
}

func TestTunneledVoiceIsCountedByFrameNumber(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	alice := dial(t, addr, "alice")
	for _, f := range []uint64{1, 2, 4, 3} {
		alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(f, []byte{0xfc})))
	}
	from := userStats(alice, alice.session, false).GetFromClient()
	if from.GetGood() != 4 || from.GetLost() != 0 || from.GetLate() != 1 {
		t.Fatalf("from client %v", from)
	}
	stats := s.ConnectionStats()
	if len(stats) != 1 || stats[0].Transport != "tcp" || stats[0].TCP != (VoiceStats{Good: 4, Late: 1}) ||
		stats[0].Bandwidth == 0 || stats[0].Name != "alice" {
		t.Fatalf("stats %+v", stats)
	}
}

func TestUDPVoiceIsCountedByTheCrypt(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	alice := dial(t, addr, "alice")
	alice.openUDP(addr)
	alice.sendUDP(audio(1, []byte{0xfc}))
	alice.sendUDP([]byte{1, 0x08, 1})
	alice.readUDP()
	stats := s.ConnectionStats()
	if len(stats) != 1 || stats[0].Transport != "udp" || stats[0].UDP.Good != 2 || stats[0].TCP.Good != 0 {
		t.Fatalf("stats %+v", stats)
	}
}

func TestWebClientsAreWebSocketConnections(t *testing.T) {
	overflows := func() map[uint32]uint64 { return map[uint32]uint64{1: 4} }
	s, _ := startServerAndListen(t, "127.0.0.1", Config{TextOverflows: overflows}, nil)
	web := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5")}, "webby"))
	web.talk(0)
	web.sync()
	stats := s.ConnectionStats()
	if len(stats) != 1 || stats[0].Transport != "websocket" || stats[0].TCP.Good != 1 ||
		stats[0].TextOverflows != 4 || stats[0].Address != netip.MustParseAddr("203.0.113.5") || stats[0].Release != "test" {
		t.Fatalf("stats %+v", stats)
	}
}

func TestPingsKeepTheClientsMeasurements(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.send(mumble.TypePing, &pb.Ping{
		Timestamp: proto.Uint64(7), Good: proto.Uint32(5), Late: proto.Uint32(1), Lost: proto.Uint32(2), Resync: proto.Uint32(1),
		TcpPackets: proto.Uint32(9), TcpPingAvg: proto.Float32(12.5), TcpPingVar: proto.Float32(3),
		UdpPackets: proto.Uint32(4), UdpPingAvg: proto.Float32(20), UdpPingVar: proto.Float32(1),
	})
	var pong pb.Ping
	proto.Unmarshal(alice.expect(mumble.TypePing), &pong)
	// As Murmur: the timestamp and the server's own counters.
	if pong.GetTimestamp() != 7 || pong.Good == nil || pong.Resync == nil || pong.TcpPingAvg != nil {
		t.Fatalf("pong %v", &pong)
	}
	alice.sync() // a ping without measurements (as the web bridge's) keeps them
	m := userStats(bob, alice.session, true)
	if m.GetTcpPingAvg() != 12.5 || m.GetTcpPingVar() != 3 || m.GetTcpPackets() != 9 ||
		m.GetUdpPingAvg() != 20 || m.GetUdpPingVar() != 1 || m.GetUdpPackets() != 4 {
		t.Fatalf("pings %v", m)
	}
	if f := m.GetFromServer(); f.GetGood() != 5 || f.GetLate() != 1 || f.GetLost() != 2 || f.GetResync() != 1 {
		t.Fatalf("from server %v", f)
	}
	if m.Onlinesecs == nil || m.Idlesecs == nil {
		t.Fatalf("times %v", m)
	}
}

// As Murmur: version and address only for oneself and admins (Register in
// root), counters only in the same channel.
func TestUserStatsDetailsNeedPermission(t *testing.T) {
	addr, _ := adminServer(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	su := superUser(t, addr)
	own := userStats(alice, alice.session, false)
	if !bytes.Equal(own.GetAddress(), netip.MustParseAddr("::ffff:127.0.0.1").AsSlice()) ||
		own.GetVersion().GetRelease() != "test" || own.GetVersion().GetVersionV2() != versionV2 || !own.GetOpus() {
		t.Fatalf("own %v", own)
	}
	if m := userStats(alice, alice.session, true); m.Address != nil || m.Version != nil {
		t.Fatalf("stats only %v", m)
	}
	if m := userStats(bob, alice.session, false); m.Address != nil || m.Version != nil || m.FromClient == nil || m.Bandwidth == nil {
		t.Fatalf("bob sees %v", m)
	}
	if m := userStats(su, alice.session, false); len(m.GetAddress()) != 16 || m.Version == nil {
		t.Fatalf("SuperUser sees %v", m)
	}
}

func TestUserStatsOfOtherChannels(t *testing.T) {
	addr, _ := adminServer(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	su := superUser(t, addr)
	games := su.createChannel(0, "Games", false)
	su.move(alice.session, games)
	bob.waitForMove(alice.session, games)
	if m := userStats(bob, alice.session, false); m.FromClient != nil || m.Bandwidth != nil || m.Onlinesecs == nil {
		t.Fatalf("bob sees %v", m)
	}
	// Without Enter there (but seeing it): denied, as Murmur.
	su.setACL(games, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Enter)}, nil)
	su.sync()
	bob.send(mumble.TypeUserStats, &pb.UserStats{Session: proto.Uint32(alice.session)})
	bob.expectDenied(pb.PermissionDenied_Permission)
}

func TestUserStatsOfHiddenUsersStayHidden(t *testing.T) {
	db := testDB(t)
	setAdminPassword(db, "admin")
	var secret uint32
	addr := startServer(t, "127.0.0.1", Config{DB: db}, func(s *Server) {
		ch, _ := s.state.AddChannel(0, "Secret", false)
		hide(ch)
		secret = ch.ID
	})
	bob := dial(t, addr, "bob")
	su := superUser(t, addr)
	su.move(su.session, secret)
	su.waitForMove(su.session, secret)
	bob.send(mumble.TypeUserStats, &pb.UserStats{Session: proto.Uint32(su.session)})
	bob.expectNone(mumble.TypeUserStats)
}
