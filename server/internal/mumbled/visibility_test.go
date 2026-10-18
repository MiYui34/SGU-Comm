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

// hide makes a channel (and its subchannels) visible only to these users.
func hide(ch *Channel, users ...uint32) {
	ch.ACLs = append(ch.ACLs, ACL{ApplyHere: true, ApplySubs: true, Group: "all", Deny: Traverse | Enter})
	for _, id := range users {
		ch.ACLs = append(ch.ACLs, ACL{ApplyHere: true, ApplySubs: true, HasUser: true, UserID: id, Allow: Traverse | Enter})
	}
}

func TestHiddenChannelsAndTheirSubchannelsAreNotVisible(t *testing.T) {
	s := newTestState(t)
	secret := mustAdd(t, s, 0, "Secret", false)
	below := mustAdd(t, s, secret.ID, "Below", false)
	open := mustAdd(t, s, 0, "Open", false)
	hide(secret, 7)
	guest := &User{Session: 1}
	member := &User{Session: 2, Registered: true, UserID: 7}
	if got := s.Visible(guest); !got[0] || !got[open.ID] || got[secret.ID] || got[below.ID] {
		t.Fatalf("guest sees %v", got)
	}
	if got := s.Visible(member); !got[secret.ID] || !got[below.ID] {
		t.Fatalf("member sees %v", got)
	}
	if s.Sees(guest, below.ID) || !s.Sees(member, below.ID) || s.Sees(member, 99) {
		t.Fatal("Sees disagrees")
	}
}

// Over links, voice from a hidden channel reaches only those who see it.
func TestHiddenChannelsAreNotHeardOverLinks(t *testing.T) {
	s := newTestState(t)
	secret := mustAdd(t, s, 0, "Secret", false)
	open := mustAdd(t, s, 0, "Open", false)
	hide(secret, 7, 8)
	s.Link(secret.ID, open.ID)
	s.AddUser(&User{Session: 1, Registered: true, UserID: 7, Channel: secret.ID})
	s.AddUser(&User{Session: 2, Channel: open.ID})
	s.AddUser(&User{Session: 3, Registered: true, UserID: 8, Channel: open.ID})
	if got := s.Listeners(1); !slices.Equal(got, []uint32{3}) {
		t.Fatalf("listeners %v", got)
	}
	if got := s.Listeners(2); !slices.Equal(got, []uint32{3}) { // no Speak in Secret
		t.Fatalf("listeners %v", got)
	}
}

// view is what a client knows from the frames it got: channels, and the
// channel of each user.
type view struct {
	channels map[uint32]bool
	users    map[uint32]uint32
}

func newView() *view { return &view{channels: map[uint32]bool{}, users: map[uint32]uint32{}} }

// update reads frames until none came for a short time.
func (v *view) update(c *testClient) *view {
	c.t.Helper()
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				return v
			}
			switch f.typ {
			case mumble.TypeChannelState:
				var m pb.ChannelState
				proto.Unmarshal(f.body, &m)
				v.channels[m.GetChannelId()] = true
			case mumble.TypeChannelRemove:
				var m pb.ChannelRemove
				proto.Unmarshal(f.body, &m)
				delete(v.channels, m.GetChannelId())
			case mumble.TypeUserState:
				var m pb.UserState
				proto.Unmarshal(f.body, &m)
				if _, known := v.users[m.GetSession()]; !known && m.Name == nil {
					c.t.Fatalf("a change for unknown user %d: %v", m.GetSession(), &m)
				}
				if m.ChannelId != nil || m.Name != nil {
					v.users[m.GetSession()] = m.GetChannelId()
				}
			case mumble.TypeUserRemove:
				var m pb.UserRemove
				proto.Unmarshal(f.body, &m)
				delete(v.users, m.GetSession())
			case mumble.TypeServerSync:
				var m pb.ServerSync
				proto.Unmarshal(f.body, &m)
				c.session = m.GetSession()
			}
		case <-time.After(300 * time.Millisecond):
			return v
		}
	}
}

// webUser joins as a Kallisti account (a registered user) and returns its
// view and user id.
func webUser(t *testing.T, s *Server, name string) (*testClient, *view, uint32) {
	t.Helper()
	c := webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: name, CertHash: name + "-hash"}}, name)
	v := newView().update(c)
	r, err := s.cfg.DB.UserByHash(name + "-hash")
	if err != nil || r == nil {
		t.Fatalf("not registered: %v", err)
	}
	return c, v, r.ID
}

// changeACL changes a channel's ACL as an ACL edit does.
func changeACL(s *Server, id uint32, change func(*Channel)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	change(s.state.Channels[id])
	s.permissionsChangedLocked(s.state.Channels[id])
}

func TestHiddenChannelsAndTheirUsersAreNotSent(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	secret := su.createChannel(0, "Secret", false)
	below := su.createChannel(secret, "Below", false)
	alice, av, aliceID := webUser(t, s, "alice")
	changeACL(s, secret, func(ch *Channel) { hide(ch, aliceID) })
	alice.move(alice.session, below)
	av.update(alice)
	if !av.channels[secret] || !av.channels[below] || av.users[alice.session] != below {
		t.Fatalf("alice sees %+v", av)
	}
	bob, bv, _ := webUser(t, s, "bob")
	if bv.channels[secret] || bv.channels[below] || !bv.channels[0] {
		t.Fatalf("bob sees channels %v", bv.channels)
	}
	if _, ok := bv.users[alice.session]; ok {
		t.Fatal("bob sees alice")
	}
	// Changes of alice reach only those who see her.
	alice.send(mumble.TypeUserState, &pb.UserState{SelfMute: proto.Bool(true)})
	bv.update(bob)
	// She comes out: bob gets all of her, and loses her again when she
	// goes back.
	alice.move(alice.session, 0)
	if bv.update(bob); bv.users[alice.session] != 0 {
		t.Fatalf("bob sees users %v", bv.users)
	}
	alice.move(alice.session, secret)
	if bv.update(bob); len(bv.users) != 2 {
		t.Fatalf("bob sees users %v", bv.users)
	}
	// Nobody else may move into it, or be moved there.
	bob.move(bob.session, secret)
	bob.expectDenied(pb.PermissionDenied_Permission)
}

func TestChannelsAppearAndGoWhenAccessChanges(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	secret := su.createChannel(0, "Secret", false)
	below := su.createChannel(secret, "Below", false)
	alice, av, aliceID := webUser(t, s, "alice")
	bob, bv, bobID := webUser(t, s, "bob")
	bob.move(bob.session, below)
	bv.update(bob)
	av.update(alice)
	// Hidden from bob while he is inside: he goes up to root, and loses
	// the channels.
	changeACL(s, secret, func(ch *Channel) { hide(ch, aliceID) })
	bv.update(bob)
	if bv.channels[secret] || bv.channels[below] || bv.users[bob.session] != 0 {
		t.Fatalf("bob sees %+v", bv)
	}
	if av.update(alice); !av.channels[below] || av.users[bob.session] != 0 {
		t.Fatalf("alice sees %+v", av)
	}
	// Alice inside; bob may see it again: the channels and alice appear.
	alice.move(alice.session, below)
	bv.update(bob)
	changeACL(s, secret, func(ch *Channel) { hide(ch, bobID) })
	if bv.update(bob); !bv.channels[secret] || !bv.channels[below] {
		t.Fatalf("bob sees %+v", bv)
	}
	// Alice is out of it now (moved up to root).
	if av.update(alice); av.users[alice.session] != 0 || av.channels[secret] {
		t.Fatalf("alice sees %+v", av)
	}
	if bv.users[alice.session] != 0 {
		t.Fatalf("bob sees users %v", bv.users)
	}
	// Removed channels: only those who saw them hear of it.
	su.send(mumble.TypeChannelRemove, &pb.ChannelRemove{ChannelId: proto.Uint32(secret)})
	alice.expectNone(mumble.TypeChannelRemove)
	if bv.update(bob); bv.channels[secret] || bv.channels[below] {
		t.Fatalf("bob sees %+v", bv)
	}
}

// Comments and avatars of hidden users, and descriptions of hidden
// channels, are not sent on request; private texts to them do not arrive.
func TestHiddenUsersCannotBeAskedAbout(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	secret := su.createChannel(0, "Secret", false)
	alice, _, aliceID := webUser(t, s, "alice")
	changeACL(s, secret, func(ch *Channel) {
		hide(ch, aliceID)
		ch.Description = "the plan"
	})
	alice.send(mumble.TypeUserState, &pb.UserState{Comment: proto.String("hello")})
	alice.move(alice.session, secret)
	bob := dial(t, addr, "bob")
	bob.send(mumble.TypeRequestBlob, &pb.RequestBlob{SessionComment: []uint32{alice.session}, ChannelDescription: []uint32{secret}})
	bob.expectNone(mumble.TypeUserState)
	bob.send(mumble.TypeTextMessage, &pb.TextMessage{Session: []uint32{alice.session}, Message: proto.String("hi")})
	bob.expectNone(mumble.TypePermissionDenied)
	alice.expectNone(mumble.TypeTextMessage)
}
