package mumbled

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// lobbyServer is a Kallisti server: root is a lobby, General a voice
// channel. SuperUser's password is "admin"; Mumble clients log in to the
// account "alice" (certificate "alice-cert") with the password "pw".
func lobbyServer(t *testing.T) (*Server, string, uint32) {
	t.Helper()
	db := testDB(t)
	setAdminPassword(db, "admin")
	if err := db.InitLobby(); err != nil {
		t.Fatal(err)
	}
	login := func(name, pw string) (*WebAccount, bool) {
		if name != "alice" {
			return nil, true
		}
		return &WebAccount{Name: "alice", CertHash: "alice-cert"}, pw == "pw"
	}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, AccountLogin: login}, nil)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.state.Channels {
		if ch.Name == "General" {
			return s, addr, ch.ID
		}
	}
	t.Fatal("no General")
	return nil, "", 0
}

// aliceTab is another browser tab of the account alice.
func aliceTab(t *testing.T, s *Server, ip string) *testClient {
	t.Helper()
	c, _ := aliceTabIn(t, s, ip)
	return c
}

// aliceTabIn also returns the channel its own UserState (before
// ServerSync) has.
func aliceTabIn(t *testing.T, s *Server, ip string) (*testClient, uint32) {
	t.Helper()
	account := &WebAccount{Name: "alice", CertHash: "alice-cert"}
	c := webClient(t, s, Origin{Host: netip.MustParseAddr(ip), Account: account}, "alice")
	channels := map[uint32]uint32{}
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				t.Fatal("connection closed")
			}
			switch f.typ {
			case mumble.TypeUserState:
				var m pb.UserState
				if proto.Unmarshal(f.body, &m) == nil && m.ChannelId != nil {
					channels[m.GetSession()] = m.GetChannelId()
				}
			case mumble.TypeServerSync:
				var m pb.ServerSync
				proto.Unmarshal(f.body, &m)
				c.session = m.GetSession()
				return c, channels[c.session]
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no ServerSync")
		}
	}
}

// neverSees fails if the client hears of the session within a short time.
func (c *testClient) neverSees(session uint32) {
	c.t.Helper()
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case f, ok := <-c.frames:
			var m pb.UserState
			if ok && f.typ == mumble.TypeUserState && proto.Unmarshal(f.body, &m) == nil && m.GetSession() == session {
				c.t.Fatalf("saw session %d: %v", session, &m)
			}
		case <-timeout:
			return
		}
	}
}

func (c *testClient) waitForRemove(session uint32) *pb.UserRemove {
	c.t.Helper()
	for {
		var m pb.UserRemove
		proto.Unmarshal(c.expect(mumble.TypeUserRemove), &m)
		if m.GetSession() == session {
			return &m
		}
	}
}

func isIn(channel uint32) func(*pb.UserState) bool {
	return func(m *pb.UserState) bool { return m.ChannelId != nil && m.GetChannelId() == channel }
}

func TestTwoTabsOfAnAccountCoexist(t *testing.T) {
	s, addr, _ := lobbyServer(t)
	carol := dial(t, addr, "carol")
	first := aliceTab(t, s, "203.0.113.1")
	carol.waitForState(first.session, isIn(0))
	second, in := aliceTabIn(t, s, "198.51.100.2")
	// The second tab is text-only: it sees itself in no channel, the
	// others never see it, and the first stays.
	if in != TextOnly {
		t.Fatalf("second tab in %d", in)
	}
	carol.neverSees(second.session)
	first.expectNone(mumble.TypeUserRemove)
}

func TestJoiningVoiceInAnotherTabMovesVoiceThere(t *testing.T) {
	s, addr, general := lobbyServer(t)
	carol := dial(t, addr, "carol")
	first := aliceTab(t, s, "203.0.113.1")
	first.move(first.session, general)
	carol.waitForMove(first.session, general)
	second := aliceTab(t, s, "203.0.113.1")
	second.move(second.session, general)
	// The first tab is told it is text-only now; others see one user
	// leave and join.
	first.waitForState(first.session, isIn(TextOnly))
	carol.waitForRemove(first.session)
	carol.waitForState(second.session, isIn(general))
	second.waitForState(second.session, isIn(general))
	if got := s.Online(); len(got) != 1 {
		t.Fatalf("online %v", got)
	}
	for _, u := range s.Online() {
		if u.Session != second.session {
			t.Fatalf("online session %d, want %d", u.Session, second.session)
		}
	}
	// Back: voice moves to the first tab again.
	first.move(first.session, general)
	second.waitForState(second.session, isIn(TextOnly))
	carol.waitForState(first.session, isIn(general))
}

func TestTextOnlySessionsNeitherHearNorSpeak(t *testing.T) {
	s, addr, general := lobbyServer(t)
	carol := dial(t, addr, "carol")
	carol.move(carol.session, general)
	carol.waitForMove(carol.session, general)
	voice := aliceTab(t, s, "203.0.113.1")
	voice.move(voice.session, general)
	carol.waitForMove(voice.session, general)
	text := aliceTab(t, s, "203.0.113.1")
	text.sync()
	carol.talk(0)
	if h := parseAudio(t, voice.expect(mumble.TypeUDPTunnel)); h.sender != carol.session {
		t.Fatalf("voice heard %+v", h)
	}
	text.expectNone(mumble.TypeUDPTunnel)
	// Nor by a whisper to its session, nor does it speak or listen.
	carol.send(mumble.TypeVoiceTarget, &pb.VoiceTarget{Id: proto.Uint32(3), Targets: []*pb.VoiceTarget_Target{{Session: []uint32{text.session}}}})
	carol.sync()
	carol.talk(3)
	text.expectNone(mumble.TypeUDPTunnel)
	text.talk(0)
	carol.expectNone(mumble.TypeUDPTunnel)
	text.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(text.session), ListeningChannelAdd: []uint32{general}})
	text.expectDenied(pb.PermissionDenied_Permission)
	// Self mute works, and only it hears of it.
	text.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(text.session), SelfMute: proto.Bool(true)})
	text.waitForState(text.session, func(m *pb.UserState) bool { return m.GetSelfMute() })
	carol.neverSees(text.session)
}

func TestAMumbleClientTakesVoice(t *testing.T) {
	s, addr, general := lobbyServer(t)
	carol := dial(t, addr, "carol")
	web := aliceTab(t, s, "203.0.113.1")
	web.move(web.session, general)
	carol.waitForMove(web.session, general)
	native := dialAs(t, addr, dialOpts{name: "alice", password: "pw", cert: clientCert(t)})
	web.waitForState(web.session, isIn(TextOnly))
	carol.waitForRemove(web.session)
	carol.waitForState(native.session, isIn(general)) // it stays where it was
	// A second Mumble client replaces the first.
	again := dialAs(t, addr, dialOpts{name: "alice", password: "pw", cert: clientCert(t)})
	native.waitForRemove(native.session)
	carol.waitForState(again.session, isIn(general))
	// Voice back to the web: a Mumble client cannot be text-only, it goes.
	web.move(web.session, general)
	if m := again.waitForRemove(again.session); m.GetReason() == "" {
		t.Fatalf("Mumble client got %v", m)
	}
	carol.waitForState(web.session, isIn(general))
}

func TestModerationHitsAllSessionsOfAnAccount(t *testing.T) {
	s, addr, general := lobbyServer(t)
	su := superUser(t, addr)
	voice := aliceTab(t, s, "203.0.113.1")
	voice.move(voice.session, general)
	su.waitForMove(voice.session, general)
	text := aliceTab(t, s, "198.51.100.2")
	text.sync()
	// A server mute stays with the account when voice moves.
	su.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(voice.session), Mute: proto.Bool(true)})
	su.waitForState(voice.session, func(m *pb.UserState) bool { return m.GetMute() })
	text.move(text.session, general)
	if m := su.waitForState(text.session, isIn(general)); !m.GetMute() {
		t.Fatalf("the new voice session is not muted: %v", m)
	}
	// A time-out mutes every session.
	if err := s.SetTimeout(su.session, text.session, time.Minute); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	for _, u := range s.state.Users {
		if u.Name == "alice" && !u.Mute {
			t.Errorf("session %d not muted", u.Session)
		}
	}
	s.mu.Unlock()
	// A ban removes all of them, and keeps the account out.
	if err := s.Ban(su.session, text.session, "spam", 0); err != nil {
		t.Fatal(err)
	}
	voice.waitForRemove(voice.session)
	text.waitForRemove(text.session)
	account := &WebAccount{Name: "alice", CertHash: "alice-cert"}
	again := webClient(t, s, Origin{Host: netip.MustParseAddr("192.0.2.3"), Account: account}, "alice")
	again.expect(mumble.TypeReject)
}

func TestABanCoversTheAddressesOfAllSessions(t *testing.T) {
	s, addr, _ := lobbyServer(t)
	su := superUser(t, addr)
	first := aliceTab(t, s, "203.0.113.1")
	second := aliceTab(t, s, "2001:db8::2")
	third := aliceTab(t, s, "203.0.113.1")
	third.sync()
	if err := s.Ban(su.session, second.session, "spam", time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*testClient{first, second, third} {
		c.waitForRemove(c.session)
	}
	var prefixes, hashes []string
	for _, b := range s.currentBans() {
		if b.Reason != "spam" || b.Duration != time.Hour {
			t.Errorf("ban %+v", b)
		}
		if b.Prefix.IsValid() {
			prefixes = append(prefixes, b.Prefix.String())
		}
		if b.Hash != "" {
			hashes = append(hashes, b.Hash)
		}
	}
	if len(hashes) != 1 { // the tabs share the account's certificate
		t.Errorf("certificate bans %v", hashes)
	}
	if len(prefixes) != 2 || !slices.Contains(prefixes, "203.0.113.1/32") || !slices.Contains(prefixes, "2001:db8::2/128") {
		t.Fatalf("address bans %v", prefixes)
	}
	for _, ip := range []string{"203.0.113.1", "2001:db8::2"} {
		if _, ok := <-webClient(t, s, Origin{Host: netip.MustParseAddr(ip)}, "bob").frames; ok {
			t.Errorf("%s got in", ip) // banned addresses are closed at once
		}
	}
}

func TestAServerMuteOutlastsTheVoiceTab(t *testing.T) {
	s, addr, general := lobbyServer(t)
	su := superUser(t, addr)
	voice := aliceTab(t, s, "203.0.113.1")
	voice.move(voice.session, general)
	su.waitForMove(voice.session, general)
	text := aliceTab(t, s, "203.0.113.1")
	text.sync()
	su.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(voice.session), Mute: proto.Bool(true)})
	su.waitForState(voice.session, func(m *pb.UserState) bool { return m.GetMute() })
	voice.conn.Close()
	su.waitForState(text.session, isIn(0)) // promoted to the lobby
	text.move(text.session, general)
	if m := su.waitForState(text.session, isIn(general)); !m.GetMute() {
		s.mu.Lock()
		muted := s.state.Users[text.session].Mute
		s.mu.Unlock()
		if !muted {
			t.Fatal("the mute was lost with the voice tab")
		}
	}
}

func TestTextFromATextOnlyTabComesFromTheVoiceSession(t *testing.T) {
	s, addr, general := lobbyServer(t)
	carol := dial(t, addr, "carol")
	carol.move(carol.session, general)
	carol.waitForMove(carol.session, general)
	voice := aliceTab(t, s, "203.0.113.1")
	voice.move(voice.session, general)
	carol.waitForMove(voice.session, general)
	text := aliceTab(t, s, "203.0.113.1")
	text.sync()
	s.SendChannelText(text.session, general, "hi")
	var m pb.TextMessage
	proto.Unmarshal(carol.expect(mumble.TypeTextMessage), &m)
	if m.GetActor() != voice.session {
		t.Fatalf("carol got %v", &m)
	}
}

func TestAKickRemovesAllSessions(t *testing.T) {
	s, addr, _ := lobbyServer(t)
	su := superUser(t, addr)
	first := aliceTab(t, s, "203.0.113.1")
	second := aliceTab(t, s, "203.0.113.1")
	second.sync()
	su.send(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(first.session), Reason: proto.String("bye")})
	first.waitForRemove(first.session)
	if m := second.waitForRemove(second.session); m.GetReason() != "bye" {
		t.Fatalf("second tab got %v", m)
	}
}

func TestAnotherTabTakesOverWhenTheVisibleOneLeaves(t *testing.T) {
	s, addr, _ := lobbyServer(t)
	carol := dial(t, addr, "carol")
	first := aliceTab(t, s, "203.0.113.1")
	carol.waitForState(first.session, isIn(0))
	second := aliceTab(t, s, "203.0.113.1")
	second.sync()
	first.conn.Close()
	carol.waitForRemove(first.session)
	carol.waitForState(second.session, isIn(0))
	second.waitForState(second.session, isIn(0))
	second.conn.Close()
	deadline := time.Now().Add(3 * time.Second)
	for len(s.Online()) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := s.Online(); len(got) != 0 {
		t.Fatalf("online after all left: %v", got)
	}
}

func TestOnlineCountsTextOnlySessions(t *testing.T) {
	s, addr, general := lobbyServer(t)
	_ = addr
	voice := aliceTab(t, s, "203.0.113.1")
	voice.move(voice.session, general)
	voice.waitForMove(voice.session, general)
	text := aliceTab(t, s, "203.0.113.1")
	text.sync()
	online := s.Online()
	if len(online) != 1 {
		t.Fatalf("online %v", online)
	}
	for _, u := range online {
		if u.Session != voice.session {
			t.Fatalf("online session %d, want the voice session %d", u.Session, voice.session)
		}
	}
	// Streams and voice chats follow the voice session only.
	if _, ok := s.UserChannel(text.session); ok {
		t.Fatal("a text-only session has a channel")
	}
	if s.MaySpeak(text.session, general) {
		t.Fatal("a text-only session may speak")
	}
	if !s.MayRead(text.session, general) {
		t.Fatal("a text-only session may not read the voice chat")
	}
}

func TestTooManySessionsOfOneAccount(t *testing.T) {
	s, _, _ := lobbyServer(t)
	for range maxSessionsPerUser {
		aliceTab(t, s, "203.0.113.1")
	}
	account := &WebAccount{Name: "alice", CertHash: "alice-cert"}
	c := webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.1"), Account: account}, "alice")
	c.expect(mumble.TypeReject)
}

func TestAWebGuestStaysSingleSession(t *testing.T) {
	s, _, _ := lobbyServer(t)
	ip := netip.MustParseAddr("203.0.113.1")
	old := joined(t, webClient(t, s, Origin{Host: ip}, "guest"))
	joined(t, webClient(t, s, Origin{Host: ip}, "guest"))
	old.waitForRemove(old.session)
}

func TestTextOnlySessionsJoinAndLeaveFromNoChannel(t *testing.T) {
	db := testDB(t)
	db.InitLobby()
	type move struct{ session, from, to uint32 }
	moves := make(chan move, 16)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnUserMoved: func(session, from, to uint32) {
		moves <- move{session, from, to}
	}}, nil)
	next := func() move {
		t.Helper()
		select {
		case m := <-moves:
			return m
		case <-time.After(3 * time.Second):
			t.Fatal("no move")
			return move{}
		}
	}
	first := aliceTab(t, s, "203.0.113.1")
	if m := next(); m != (move{first.session, NoChannel, 0}) {
		t.Fatalf("first joined: %+v", m)
	}
	second := aliceTab(t, s, "203.0.113.1")
	if m := next(); m != (move{second.session, NoChannel, NoChannel}) {
		t.Fatalf("second joined: %+v", m)
	}
	second.conn.Close()
	if m := next(); m != (move{second.session, NoChannel, NoChannel}) {
		t.Fatalf("second left: %+v", m)
	}
}

// userIn is the channel of a session and whether that channel exists
// (TextOnly counts as existing).
func userIn(s *Server, session uint32) (uint32, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[session]
	if u == nil {
		return 0, false
	}
	return u.Channel, u.textOnly() || s.state.Channels[u.Channel] != nil
}

func TestAMumbleClientTakesVoiceInATemporaryChannel(t *testing.T) {
	s, addr, general := lobbyServer(t)
	web := aliceTab(t, s, "203.0.113.1")
	tmp := web.createChannel(general, "Tmp", true)
	web.waitForMove(web.session, tmp)
	native := dialAs(t, addr, dialOpts{name: "alice", password: "pw", cert: clientCert(t)})
	web.waitForState(web.session, isIn(TextOnly))
	if ch, ok := userIn(s, native.session); ch != tmp || !ok {
		t.Fatalf("Mumble client in %d (exists: %v), want %d", ch, ok, tmp)
	}
	s.mu.Lock()
	s.permissionsChangedLocked(nil) // walks users' channels: crashed
	s.mu.Unlock()
}

func TestMakingATemporaryChannelTakesVoice(t *testing.T) {
	s, addr, general := lobbyServer(t)
	su := superUser(t, addr)
	voice := aliceTab(t, s, "203.0.113.1")
	voice.move(voice.session, general)
	su.waitForMove(voice.session, general)
	su.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(voice.session), Mute: proto.Bool(true)})
	su.waitForState(voice.session, func(m *pb.UserState) bool { return m.GetMute() })
	text := aliceTab(t, s, "203.0.113.1")
	tmp := text.createChannel(general, "Tmp", true)
	voice.waitForState(voice.session, isIn(TextOnly))
	text.waitForMove(text.session, tmp)
	s.mu.Lock()
	muted := s.state.Users[text.session].Mute
	s.mu.Unlock()
	if !muted {
		t.Fatal("the server mute did not go along")
	}
}

func TestATabTakesVoiceInTheTemporaryChannelOfAnother(t *testing.T) {
	s, _, general := lobbyServer(t)
	first := aliceTab(t, s, "203.0.113.1")
	tmp := first.createChannel(general, "Tmp", true)
	first.waitForMove(first.session, tmp)
	second := aliceTab(t, s, "203.0.113.1")
	second.move(second.session, tmp)
	first.waitForState(first.session, isIn(TextOnly))
	second.waitForMove(second.session, tmp)
	if ch, ok := userIn(s, second.session); ch != tmp || !ok {
		t.Fatalf("second tab in %d (exists: %v), want %d", ch, ok, tmp)
	}
}
