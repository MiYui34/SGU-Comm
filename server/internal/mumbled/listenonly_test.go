package mumbled

import (
	"slices"
	"sync/atomic"
	"testing"

	"kallisti.chat/server/internal/password"
)

func TestModeratorsMakeAChannelListenOnly(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	alice, aliceID := registered(t, addr, "alice")
	s.SetModerator(su.session, aliceID, true)
	bob, _ := registered(t, addr, "bob")
	juke, jukeID := registered(t, addr, "jukebox")
	music := alice.createChannel(0, "Music", false)
	s.mu.Lock()
	before := slices.Clone(s.state.Channels[music].ACLs) // the creator's entry
	s.mu.Unlock()

	if err := s.SetListenOnly(alice.session, music, true, []uint32{jukeID}); err != nil {
		t.Fatal(err)
	}
	if got := s.ListenOnly(); len(got) != 1 || got[0].Channel != music || !slices.Equal(got[0].Speakers, []Speaker{{jukeID, "jukebox"}}) {
		t.Fatalf("listen-only %+v", got)
	}
	s.mu.Lock()
	bobSpeaks := s.state.Has(s.state.Users[bob.session], music, Speak)
	jukeSpeaks := s.state.Has(s.state.Users[juke.session], music, Speak)
	aliceSpeaks := s.state.Has(s.state.Users[alice.session], music, Speak)
	s.mu.Unlock()
	if bobSpeaks || aliceSpeaks || !jukeSpeaks {
		t.Fatalf("speak: bob %v, alice %v, jukebox %v", bobSpeaks, aliceSpeaks, jukeSpeaks)
	}

	// Off again: only the listen-only entries go.
	if err := s.SetListenOnly(alice.session, music, false, nil); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	after := s.state.Channels[music].ACLs
	s.mu.Unlock()
	if !slices.Equal(after, before) || len(s.ListenOnly()) != 0 {
		t.Fatalf("ACL %+v, want %+v", after, before)
	}
}

func TestListenOnlyNeedsWriteAndAVoiceChannel(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	bob, _ := registered(t, addr, "bob")
	music := su.createChannel(0, "Music", false)
	if err := s.SetListenOnly(bob.session, music, true, nil); err == nil {
		t.Fatal("a user without Write made it listen-only")
	}
	if err := s.SetListenOnly(su.session, 0, true, nil); err == nil {
		t.Fatal("root made listen-only")
	}
	ids, _ := s.SyncTextChannels([]TextMirror{{Name: "general"}})
	if err := s.SetListenOnly(su.session, ids[0], true, nil); err == nil {
		t.Fatal("a mirror made listen-only")
	}
	// Unknown users are dropped.
	if err := s.SetListenOnly(su.session, music, true, []uint32{999}); err != nil {
		t.Fatal(err)
	}
	if got := s.ListenOnly(); len(got) != 1 || len(got[0].Speakers) != 0 {
		t.Fatalf("listen-only %+v", got)
	}
}

func TestListenOnlyChangesAreReported(t *testing.T) {
	var calls atomic.Int32
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnListenOnlyChanged: func() { calls.Add(1) }}, nil)
	su := superUser(t, addr)
	music := su.createChannel(0, "Music", false)
	s.SetListenOnly(su.session, music, true, nil)
	if calls.Load() != 1 {
		t.Fatalf("%d calls", calls.Load())
	}
	// A Mumble client's ACL editor too.
	su.setACL(music, true, nil, nil)
	eventually(t, func() bool { return calls.Load() == 2 })
	if len(s.ListenOnly()) != 0 {
		t.Fatal("still listen-only")
	}
}

// A deny from a Mumble client's ACL editor (also for subchannels) goes
// too when it is turned off; its other bits stay.
func TestListenOnlyOffClearsADenyFromAMumbleClient(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	music := su.createChannel(0, "Music", false)
	s.mu.Lock()
	ch := s.state.Channels[music]
	ch.ACLs = append(ch.ACLs, ACL{ApplyHere: true, ApplySubs: true, Group: "all", Deny: Speak | Whisper})
	s.mu.Unlock()
	if len(s.ListenOnly()) != 1 {
		t.Fatal("not listen-only")
	}
	s.SetListenOnly(su.session, music, false, nil)
	s.mu.Lock()
	acls := slices.Clone(ch.ACLs)
	s.mu.Unlock()
	if len(s.ListenOnly()) != 0 || !slices.Contains(acls, ACL{ApplyHere: true, ApplySubs: true, Group: "all", Deny: Whisper}) {
		t.Fatalf("ACL %+v", acls)
	}
}

func TestListenOnlyHasALimit(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	music := su.createChannel(0, "Music", false)
	if err := s.SetListenOnly(su.session, music, true, make([]uint32, maxACLEntries+1)); err == nil {
		t.Fatal("no limit")
	}
}
