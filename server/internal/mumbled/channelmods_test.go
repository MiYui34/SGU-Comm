package mumbled

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// clubServer: SuperUser made Club (with Club/Sub) and Other; alice is
// Club's moderator. roles counts OnRolesChanged calls.
type clubServer struct {
	s                *Server
	addr             string
	db               *DB
	su, alice        *testClient
	aliceID          uint32
	club, sub, other uint32
	roles            *atomic.Int32
}

func newClubServer(t *testing.T) clubServer {
	t.Helper()
	db := testDB(t)
	setAdminPassword(db, "admin")
	roles := &atomic.Int32{}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnRolesChanged: func() { roles.Add(1) }}, nil)
	c := clubServer{s: s, addr: addr, db: db, roles: roles}
	c.su = superUser(t, addr)
	c.club = c.su.createChannel(0, "Club", false)
	c.sub = c.su.createChannel(c.club, "Sub", false)
	c.other = c.su.createChannel(0, "Other", false)
	c.alice, c.aliceID = registered(t, addr, "alice")
	if err := s.SetChannelModerator(c.su.session, c.aliceID, c.club, true); err != nil {
		t.Fatal(err)
	}
	return c
}

// in moves a user into a channel (by the server: clients send few
// messages at once) and waits for it.
func (c clubServer) in(u *testClient, channel uint32) { moveTo(c.s, u, channel) }

func moveTo(s *Server, u *testClient, channel uint32) {
	s.mu.Lock()
	s.moveLocked(s.state.Users[u.session], channel, 0)
	s.mu.Unlock()
	u.waitForMove(u.session, channel)
}

func TestChannelModeratorsAreAppointedByStaff(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	bob, bobID := registered(t, c.addr, "bob")
	mod, modID := registered(t, c.addr, "mod")
	s.SetModerator(c.su.session, modID, true)
	guest := dial(t, c.addr, "guest")
	for _, e := range []struct {
		name          string
		actor, target uint32
		channel       uint32
	}{
		{"a plain user", bob.session, bobID, c.club},
		{"a guest", guest.session, bobID, c.club},
		{"a channel moderator", c.alice.session, bobID, c.club},
		{"SuperUser", c.su.session, 0, c.club},
		{"an unknown user", c.su.session, 999, c.club},
		{"root", c.su.session, bobID, 0},
		{"an unknown channel", c.su.session, bobID, 999},
	} {
		if err := s.SetChannelModerator(e.actor, e.target, e.channel, true); err == nil {
			t.Errorf("%s: no error", e.name)
		}
	}
	// Server moderators appoint them too.
	before := c.roles.Load()
	if err := s.SetChannelModerator(mod.session, bobID, c.other, true); err != nil {
		t.Fatal(err)
	}
	if c.roles.Load() == before {
		t.Fatal("no roles event")
	}
	want := []ChannelModerators{{Channel: c.club, Users: []uint32{c.aliceID}}, {Channel: c.other, Users: []uint32{bobID}}}
	if got := s.ChannelModerators(); !slices.EqualFunc(got, want, func(a, b ChannelModerators) bool {
		return a.Channel == b.Channel && slices.Equal(a.Users, b.Users)
	}) {
		t.Fatalf("channel moderators %+v", got)
	}
	// The last one removed: the group and its ACL entry go.
	if err := s.SetChannelModerator(mod.session, bobID, c.other, false); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	other := s.state.Channels[c.other]
	left := len(other.ACLs) > 0 || other.Groups[channelModGroup] != nil
	s.mu.Unlock()
	if left || len(s.ChannelModerators()) != 1 {
		t.Fatalf("after removing: %+v %v", other, s.ChannelModerators())
	}
}

func TestChannelModeratorsModerateTheirChannelAndSubchannels(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	for _, e := range []struct {
		channel uint32
		want    bool
	}{{c.club, true}, {c.sub, true}, {c.other, false}, {0, false}} {
		if got := s.ModeratesIn(c.alice.session, e.channel); got != e.want {
			t.Errorf("channel %d: %v", e.channel, got)
		}
	}
	if !s.ModeratesIn(c.su.session, c.other) {
		t.Error("staff moderate everywhere")
	}
	guest := dial(t, c.addr, "guest")
	// Mute and move out (the voice kick: to the lobby) in Club and Sub.
	c.in(guest, c.club)
	c.alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(guest.session), Mute: proto.Bool(true)})
	guest.waitForState(guest.session, muted(true))
	c.alice.move(guest.session, 0)
	guest.waitForMove(guest.session, 0)
	c.in(guest, c.sub)
	c.alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(guest.session), Mute: proto.Bool(false)})
	guest.waitForState(guest.session, muted(false))
	c.alice.move(guest.session, 0)
	guest.waitForMove(guest.session, 0)
}

func TestChannelModeratorsDoNothingElsewhere(t *testing.T) {
	c := newClubServer(t)
	guest := dial(t, c.addr, "guest")
	// Not in other channels, and nobody is moved into theirs from elsewhere.
	c.in(guest, c.other)
	c.alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(guest.session), Mute: proto.Bool(true)})
	c.alice.expectDenied(pb.PermissionDenied_Permission)
	c.alice.move(guest.session, c.club)
	c.alice.expectDenied(pb.PermissionDenied_Permission)
	// No kicks or bans: those are server-wide; no ACL edits in their own
	// channel (they would appoint moderators).
	c.in(guest, c.club)
	c.alice.send(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(guest.session)})
	c.alice.expectDenied(pb.PermissionDenied_Permission)
	c.alice.setACL(c.club, true, nil, nil)
	c.alice.expectDenied(pb.PermissionDenied_Permission)
}

func TestChannelModeratorsDoNotActOnStaffOrEachOther(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	mod, modID := registered(t, c.addr, "mod")
	s.SetModerator(c.su.session, modID, true)
	dave, daveID := registered(t, c.addr, "dave")
	s.SetChannelModerator(c.su.session, daveID, c.club, true)
	for _, u := range []*testClient{mod, dave, c.su} {
		c.in(u, c.club)
		c.alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(u.session), Mute: proto.Bool(true)})
		c.alice.expect(mumble.TypePermissionDenied)
		if err := s.SetChannelTimeout(c.alice.session, u.session, c.club, time.Hour); err == nil {
			t.Errorf("%d: timed out", u.session)
		}
	}
	c.alice.move(dave.session, 0)
	c.alice.expectDenied(pb.PermissionDenied_Permission)
	// Server moderators act on channel moderators.
	mod.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(dave.session), Mute: proto.Bool(true)})
	dave.waitForState(dave.session, muted(true))
	if err := s.SetChannelTimeout(mod.session, dave.session, c.club, time.Hour); err != nil {
		t.Fatal(err)
	}
}

func TestChannelModeratorsManagePrivateMembers(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	_, bobID := registered(t, c.addr, "bob")
	if err := s.SetPrivate(c.alice.session, c.club, true, nil); err == nil {
		t.Fatal("a channel moderator made the channel private")
	}
	s.SetPrivate(c.su.session, c.club, true, nil)
	if !s.Sees(c.alice.session, c.club) {
		t.Fatal("the moderator does not see the private channel")
	}
	if err := s.SetPrivate(c.alice.session, c.club, true, []uint32{bobID}); err != nil {
		t.Fatal(err)
	}
	if got := s.Private(); len(got) != 1 || len(got[0].Members) != 1 || got[0].Members[0].ID != bobID {
		t.Fatalf("private %+v", got)
	}
	for _, e := range []struct {
		channel uint32
		on      bool
	}{{c.club, false}, {c.other, true}} {
		if err := s.SetPrivate(c.alice.session, e.channel, e.on, nil); err == nil {
			t.Errorf("channel %d private %v: no error", e.channel, e.on)
		}
	}
}

func TestChannelModeratorsLastOverRestarts(t *testing.T) {
	c := newClubServer(t)
	restarted, err := New(Config{DB: c.db})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	got := restarted.ChannelModerators()
	if len(got) != 1 || got[0].Channel != c.club || !slices.Equal(got[0].Users, []uint32{c.aliceID}) {
		t.Fatalf("after a restart: %+v", got)
	}
}

func suppressed(on bool) func(*pb.UserState) bool {
	return func(m *pb.UserState) bool { return m.Suppress != nil && m.GetSuppress() == on }
}

func TestChannelTimeOutRules(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	bob := dial(t, c.addr, "bob")
	guest := dial(t, c.addr, "guest")
	for _, e := range []struct {
		name          string
		actor, target uint32
		channel       uint32
		d             time.Duration
	}{
		{"a plain user", bob.session, guest.session, c.club, time.Hour},
		{"another channel", c.alice.session, guest.session, c.other, time.Hour},
		{"root", c.su.session, guest.session, 0, time.Hour},
		{"oneself", c.alice.session, c.alice.session, c.club, time.Hour},
		{"too long", c.alice.session, guest.session, c.club, maxTimeout + time.Hour},
		{"not timed out", c.alice.session, guest.session, c.club, 0},
	} {
		if err := s.SetChannelTimeout(e.actor, e.target, e.channel, e.d); err == nil {
			t.Errorf("%s: no error", e.name)
		}
	}
	if err := s.SetChannelTimeout(c.alice.session, guest.session, c.sub, time.Hour); err != nil {
		t.Fatal(err)
	}
}

func TestChannelTimedOutUsersCannotSpeakOrWriteThere(t *testing.T) {
	db := testDB(t)
	setAdminPassword(db, "admin")
	changed := &atomic.Int32{}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnTimeoutsChanged: func() { changed.Add(1) }}, nil)
	su := superUser(t, addr)
	club := su.createChannel(0, "Club", false)
	sub := su.createChannel(club, "Sub", false)
	other := su.createChannel(0, "Other", false)
	alice, aliceID := registered(t, addr, "alice")
	s.SetChannelModerator(su.session, aliceID, club, true)
	bob, bobID := registered(t, addr, "bob")
	moveTo(s, bob, club)
	if err := s.SetChannelTimeout(alice.session, bob.session, club, time.Hour); err != nil {
		t.Fatal(err)
	}
	bob.waitForState(bob.session, suppressed(true))
	if until, ok := s.ChannelTimedOut(bob.session, sub); !ok || time.Until(until) < 59*time.Minute {
		t.Fatalf("timed out in Sub: %v %v", until, ok)
	}
	if _, ok := s.TimedOut(bob.session); ok || len(s.Timeouts()) != 0 {
		t.Fatal("a server time-out")
	}
	if got := s.ChannelTimeouts(); len(got) != 1 || got[0].UserID != bobID || got[0].Channel != club {
		t.Fatalf("channel time-outs %+v", got)
	}
	if changed.Load() != 1 {
		t.Fatalf("changed %d times", changed.Load())
	}
	for _, ch := range []uint32{club, sub} {
		bob.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{ch}, Message: proto.String("hi")})
		bob.expectDenied(pb.PermissionDenied_Text)
		if s.MayWrite(bob.session, ch) || s.MaySpeak(bob.session, ch) {
			t.Fatalf("bob may write or speak in %d", ch)
		}
	}
	if !s.MayWrite(bob.session, other) {
		t.Fatal("bob may not write in Other")
	}
	// Elsewhere it may speak again.
	moveTo(s, bob, other)
	if !s.MaySpeak(bob.session, other) {
		t.Error("bob may not speak in Other")
	}
	moveTo(s, bob, sub)
	s.mu.Lock()
	quiet := s.state.Users[bob.session].Suppress
	s.mu.Unlock()
	if !quiet {
		t.Error("not suppressed in Sub")
	}

	// It lasts over a restart; it ends.
	restarted, err := New(Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	if got := restarted.ChannelTimeouts(); len(got) != 1 || got[0].UserID != bobID || got[0].Channel != club {
		t.Fatalf("after a restart: %+v", got)
	}
	restarted.Close()
	if err := s.SetChannelTimeout(alice.session, bob.session, club, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	bob.waitForState(bob.session, suppressed(false))
	if len(s.ChannelTimeouts()) != 0 {
		t.Fatalf("channel time-outs %+v", s.ChannelTimeouts())
	}
	if saved, _ := db.ChannelTimeouts(); len(saved) != 0 {
		t.Fatalf("saved %v", saved)
	}
}

func TestGuestChannelTimeOutsEndWhenTheyLeave(t *testing.T) {
	c := newClubServer(t)
	guest := dial(t, c.addr, "guest")
	c.s.SetChannelTimeout(c.alice.session, guest.session, c.club, time.Hour)
	guest.conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	for len(c.s.ChannelTimeouts()) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("channel time-outs %+v", c.s.ChannelTimeouts())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestChannelTimedOutUsersCannotUnsuppressThemselves(t *testing.T) {
	c := newClubServer(t)
	c.in(c.alice, c.club)
	if err := c.s.SetChannelTimeout(c.su.session, c.alice.session, c.club, time.Hour); err != nil {
		t.Fatal(err)
	}
	c.alice.waitForState(c.alice.session, suppressed(true))
	c.alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(c.alice.session), Suppress: proto.Bool(false)})
	c.alice.expectDenied(pb.PermissionDenied_Text)
}

func TestOnlyStaffChangeChannelModeratorsInACLs(t *testing.T) {
	c := newClubServer(t)
	bob, bobID := registered(t, c.addr, "bob")
	// Bob may edit Other's ACL (as a temporary channel's maker may his).
	changeACL(c.s, c.other, func(ch *Channel) {
		ch.ACLs = append(ch.ACLs, ACL{ApplyHere: true, HasUser: true, UserID: bobID, Allow: Write})
	})
	group := &pb.ACL_ChanGroup{Name: proto.String(channelModGroup), Inherit: proto.Bool(true), Inheritable: proto.Bool(true), Add: []uint32{bobID}}
	bob.setACL(c.other, true, []*pb.ACL_ChanACL{{UserId: proto.Uint32(bobID), ApplyHere: proto.Bool(true), Grant: proto.Uint32(uint32(Write))}}, []*pb.ACL_ChanGroup{group})
	bob.setACL(c.club, true, nil, nil) // a round trip: denied, so the edit before is done
	bob.expectDenied(pb.PermissionDenied_Permission)
	if c.s.ModeratesIn(bob.session, c.other) || len(c.s.ChannelModerators()) != 1 {
		t.Fatalf("bob made himself a moderator: %+v", c.s.ChannelModerators())
	}
	// Staff may.
	c.su.setACL(c.other, true, nil, []*pb.ACL_ChanGroup{group})
	deadline := time.Now().Add(2 * time.Second)
	for !c.s.ModeratesIn(bob.session, c.other) {
		if time.Now().After(deadline) {
			t.Fatal("SuperUser's edit did not make bob a moderator")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestChannelModeratorsDoNothingInPrivateSubchannelsTheyDoNotSee(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	guest, guestID := registered(t, c.addr, "member")
	s.SetPrivate(c.su.session, c.sub, true, []uint32{guestID})
	c.in(guest, c.sub)
	if s.ModeratesIn(c.alice.session, c.sub) {
		t.Fatal("moderates a private subchannel")
	}
	if err := s.SetPrivate(c.alice.session, c.sub, true, []uint32{c.aliceID}); err == nil {
		t.Fatal("joined a private subchannel")
	}
	if err := s.SetChannelTimeout(c.alice.session, guest.session, c.sub, time.Hour); err == nil {
		t.Fatal("timed out in a private subchannel")
	}
}
