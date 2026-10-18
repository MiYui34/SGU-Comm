package mumbled

import (
	"slices"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

func TestStaffMakeAChannelPrivate(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	club := su.createChannel(0, "Club", false)
	below := su.createChannel(club, "Below", false)
	mod, mv, modID := webUser(t, s, "mod")
	s.SetModerator(su.session, modID, true)
	alice, av, aliceID := webUser(t, s, "alice")
	bob, bv, _ := webUser(t, s, "bob")
	s.mu.Lock()
	before := slices.Clone(s.state.Channels[club].ACLs)
	s.mu.Unlock()

	if err := s.SetPrivate(bob.session, club, true, nil); err == nil {
		t.Fatal("a user made it private")
	}
	if err := s.SetPrivate(mod.session, club, true, []uint32{aliceID, 999}); err != nil {
		t.Fatal(err)
	}
	if got := s.Private(); len(got) != 1 || got[0].Channel != club || !slices.Equal(got[0].Members, []Member{{aliceID, "alice"}}) {
		t.Fatalf("private %+v", got)
	}
	av.update(alice)
	mv.update(mod)
	bv.update(bob)
	if !av.channels[below] || !mv.channels[below] || bv.channels[club] || bv.channels[below] {
		t.Fatalf("alice %v, mod %v, bob %v", av.channels, mv.channels, bv.channels)
	}
	s.mu.Lock()
	aliceSpeaks := s.state.Has(s.state.Users[alice.session], club, Enter|Speak|TextMessage)
	bobPerms := s.state.Permissions(s.state.Users[bob.session], club)
	s.mu.Unlock()
	if !aliceSpeaks || bobPerms != 0 {
		t.Fatalf("alice may speak %v, bob has %x", aliceSpeaks, bobPerms)
	}

	// Members change: bob in, alice out.
	if err := s.SetPrivate(mod.session, club, true, []uint32{bob.userID(t, s)}); err != nil {
		t.Fatal(err)
	}
	if av.update(alice); av.channels[club] {
		t.Fatal("alice still sees it")
	}
	if bv.update(bob); !bv.channels[club] {
		t.Fatal("bob does not see it")
	}

	// Public again: only the private entries and the group go.
	if err := s.SetPrivate(mod.session, club, false, nil); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	after, groups := s.state.Channels[club].ACLs, s.state.Channels[club].Groups
	s.mu.Unlock()
	if !slices.Equal(after, before) || groups[membersGroup] != nil || len(s.Private()) != 0 {
		t.Fatalf("ACL %+v, groups %v", after, groups)
	}
	if av.update(alice); !av.channels[club] {
		t.Fatal("alice does not see it again")
	}
}

func (c *testClient) userID(t *testing.T, s *Server) uint32 {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Users[c.session].UserID
}

func TestRootAndTheMirrorParentCannotBePrivate(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	if err := s.SetPrivate(su.session, 0, true, nil); err == nil {
		t.Fatal("root made private")
	}
	s.SyncTextChannels([]TextMirror{{Name: "general"}})
	s.mu.Lock()
	parent := s.textParent
	s.mu.Unlock()
	if err := s.SetPrivate(su.session, parent, true, nil); err == nil {
		t.Fatal("the mirror parent made private")
	}
	if err := s.SetPrivate(su.session, 99, true, nil); err == nil {
		t.Fatal("an unknown channel made private")
	}
}

// A private text channel's mirror: members and staff see it and may write
// there (staff have no Write in mirrors, so they need their own entries).
func TestPrivateMirrors(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	mod, _, modID := webUser(t, s, "mod")
	s.SetModerator(su.session, modID, true)
	alice, av, aliceID := webUser(t, s, "alice")
	bob, bv, _ := webUser(t, s, "bob")
	ids, err := s.SyncTextChannels([]TextMirror{{Name: "general"}, {Name: "secret", Private: true}})
	if err != nil {
		t.Fatal(err)
	}
	secret := ids[1]
	if bv.update(bob); bv.channels[secret] || !bv.channels[ids[0]] {
		t.Fatalf("bob sees %v", bv.channels)
	}
	if av.update(alice); av.channels[secret] {
		t.Fatal("alice sees it before she is a member")
	}
	if err := s.SetPrivate(mod.session, secret, true, []uint32{aliceID}); err != nil {
		t.Fatal(err)
	}
	if av.update(alice); !av.channels[secret] {
		t.Fatal("alice does not see it")
	}
	for _, c := range []struct {
		who       *testClient
		may       bool
		mayManage bool
	}{{alice, true, false}, {mod, true, true}, {su, true, true}, {bob, false, false}} {
		if s.Sees(c.who.session, secret) != c.may || s.MayWrite(c.who.session, secret) != c.may {
			t.Fatalf("session %d: sees %v, writes %v", c.who.session, s.Sees(c.who.session, secret), s.MayWrite(c.who.session, secret))
		}
	}
	// Still nobody speaks there.
	s.mu.Lock()
	speaks := s.state.Has(s.state.Users[alice.session], secret, Speak)
	s.mu.Unlock()
	if speaks {
		t.Fatal("alice may speak in a mirror")
	}
	// Mirrors that exist keep their access on a later sync.
	s.SyncTextChannels([]TextMirror{{Channel: ids[0], Name: "general"}, {Channel: secret, Name: "secret"}})
	if s.Sees(bob.session, secret) || len(s.Private()) != 1 {
		t.Fatal("the mirror became public")
	}
}

func TestStaffCreatePrivateChannels(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	bob, bv, _ := webUser(t, s, "bob")
	if _, err := s.CreatePrivateChannel(bob.session, "Nope"); err == nil {
		t.Fatal("a user made a private channel")
	}
	id, err := s.CreatePrivateChannel(su.session, "Club")
	if err != nil {
		t.Fatal(err)
	}
	var m pb.ChannelState
	proto.Unmarshal(su.expect(mumble.TypeChannelState), &m)
	if m.GetChannelId() != id || m.GetName() != "Club" {
		t.Fatalf("su got %v", &m)
	}
	if bv.update(bob); bv.channels[id] {
		t.Fatal("bob sees it")
	}
	if got := s.Private(); len(got) != 1 || got[0].Channel != id || len(got[0].Members) != 0 {
		t.Fatalf("private %+v", got)
	}
	if _, err := s.CreatePrivateChannel(su.session, "Club"); err == nil {
		t.Fatal("two channels of one name")
	}
}

func TestPrivateChannelsAreSavedAndReported(t *testing.T) {
	var calls atomic.Int32
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnPrivateChanged: func() { calls.Add(1) }}, nil)
	su := superUser(t, addr)
	club := su.createChannel(0, "Club", false)
	s.SetPrivate(su.session, club, true, nil)
	other, _ := s.CreatePrivateChannel(su.session, "Other")
	if calls.Load() != 2 {
		t.Fatalf("%d calls", calls.Load())
	}
	// A Mumble client's ACL editor too.
	su.setACL(club, true, nil, nil)
	eventually(t, func() bool { return calls.Load() == 3 })
	reloaded, err := New(Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Private(); len(got) != 1 || got[0].Channel != other {
		t.Fatalf("after a restart %+v", got)
	}
	// Removed: the private channels changed too.
	su.send(mumble.TypeChannelRemove, &pb.ChannelRemove{ChannelId: proto.Uint32(other)})
	eventually(t, func() bool { return calls.Load() == 4 })
}

// An invite link adds a member (the Kallisti server checks the link).
func TestAddMember(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	club := su.createChannel(0, "Club", false)
	public := su.createChannel(0, "Public", false)
	alice, av, aliceID := webUser(t, s, "alice")
	s.SetPrivate(su.session, club, true, nil)
	if added, err := s.AddMember(club, aliceID); err != nil || !added {
		t.Fatalf("added %v, %v", added, err)
	}
	if av.update(alice); !av.channels[club] {
		t.Fatal("alice does not see it")
	}
	if added, err := s.AddMember(club, aliceID); err != nil || added {
		t.Fatalf("added again %v, %v", added, err)
	}
	if _, err := s.AddMember(public, aliceID); err == nil {
		t.Fatal("a member of a public channel")
	}
	if _, err := s.AddMember(club, 999); err == nil {
		t.Fatal("an unknown user added")
	}
	reloaded, err := New(Config{DB: s.cfg.DB})
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Private(); len(got) != 1 || !slices.Equal(got[0].Members, []Member{{aliceID, "alice"}}) {
		t.Fatalf("after a restart %+v", got)
	}
}
