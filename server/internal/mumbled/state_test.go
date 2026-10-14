package mumbled

import (
	"errors"
	"slices"
	"testing"

	"kallisti.chat/server/internal/mumble/pb"
)

func newTestState(t *testing.T) *State {
	t.Helper()
	return NewState("Root", nil)
}

func mustAdd(t *testing.T, s *State, parent uint32, name string, temporary bool) *Channel {
	t.Helper()
	c, err := s.AddChannel(parent, name, temporary)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func denyType(err error) pb.PermissionDenied_DenyType {
	var d *DenyError
	if errors.As(err, &d) {
		return d.Type
	}
	return -1
}

func TestChannelsHaveUniqueValidNamesAmongSiblings(t *testing.T) {
	s := newTestState(t)
	games := mustAdd(t, s, 0, "Games", false)
	mustAdd(t, s, games.ID, "Lobby", false)
	mustAdd(t, s, 0, "Lobby", false) // another parent: fine
	if _, err := s.AddChannel(0, "lobby", false); denyType(err) != pb.PermissionDenied_ChannelName {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := s.AddChannel(0, " ", false); denyType(err) != pb.PermissionDenied_ChannelName {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := s.AddChannel(99, "Nowhere", false); err == nil {
		t.Fatal("unknown parent")
	}
}

func TestTreeListsParentsBeforeChildren(t *testing.T) {
	s := newTestState(t)
	a := mustAdd(t, s, 0, "A", false)
	b := mustAdd(t, s, a.ID, "B", false)
	mustAdd(t, s, b.ID, "C", false)
	// Move A below a channel made later: the order must follow the tree.
	d := mustAdd(t, s, 0, "D", false)
	if err := s.EditChannel(a.ID, ChannelEdit{Parent: &d.ID}); err != nil {
		t.Fatal(err)
	}
	seen := map[uint32]bool{}
	for _, c := range s.Tree() {
		if c.ID != 0 && !seen[c.Parent] {
			t.Fatalf("channel %s before its parent", c.Name)
		}
		seen[c.ID] = true
	}
}

func TestChannelsCannotMoveIntoThemselves(t *testing.T) {
	s := newTestState(t)
	a := mustAdd(t, s, 0, "A", false)
	b := mustAdd(t, s, a.ID, "B", false)
	if err := s.EditChannel(a.ID, ChannelEdit{Parent: &b.ID}); err == nil {
		t.Fatal("moved a channel below its own child")
	}
	root := uint32(0)
	if err := s.EditChannel(0, ChannelEdit{Parent: &root}); err == nil {
		t.Fatal("moved the root")
	}
}

func TestRemovingAChannelMovesItsUsersUp(t *testing.T) {
	s := newTestState(t)
	a := mustAdd(t, s, 0, "A", false)
	b := mustAdd(t, s, a.ID, "B", false)
	s.AddUser(&User{Session: 1, Name: "alice", Channel: b.ID})
	moved, removed, err := s.RemoveChannel(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(removed, []uint32{b.ID, a.ID}) {
		t.Fatalf("removed %v, want children first", removed)
	}
	if moved[1] != 0 || s.Users[1].Channel != 0 {
		t.Fatalf("alice moved to %v", moved)
	}
	if _, _, err := s.RemoveChannel(0); err == nil {
		t.Fatal("removed the root")
	}
}

func TestTemporaryChannelsGoAwayWhenEmpty(t *testing.T) {
	s := newTestState(t)
	tmp := mustAdd(t, s, 0, "Tmp", true)
	s.AddUser(&User{Session: 1, Name: "alice", Channel: 0})
	s.AddUser(&User{Session: 2, Name: "bob", Channel: 0})
	if gone := s.MoveUser(1, tmp.ID); len(gone) != 0 {
		t.Fatalf("removed %v", gone)
	}
	s.MoveUser(2, tmp.ID)
	if gone := s.MoveUser(1, 0); len(gone) != 0 {
		t.Fatal("removed while bob is in it")
	}
	if gone := s.RemoveUser(2); !slices.Equal(gone, []uint32{tmp.ID}) {
		t.Fatalf("removed %v after the last user left", gone)
	}
	if s.Channels[tmp.ID] != nil {
		t.Fatal("still there")
	}
}

func TestLinksAreBothWays(t *testing.T) {
	s := newTestState(t)
	a := mustAdd(t, s, 0, "A", false)
	b := mustAdd(t, s, 0, "B", false)
	s.Link(a.ID, b.ID)
	if !s.Channels[b.ID].Links[a.ID] {
		t.Fatal("link is one way")
	}
	s.Unlink(b.ID, a.ID)
	if s.Channels[a.ID].Links[b.ID] {
		t.Fatal("still linked")
	}
	s.Link(a.ID, b.ID)
	s.RemoveChannel(b.ID)
	if len(s.Channels[a.ID].Links) != 0 {
		t.Fatal("link to a removed channel")
	}
}

func TestVoiceGoesToTheChannelAndLinkedChannels(t *testing.T) {
	s := newTestState(t)
	a := mustAdd(t, s, 0, "A", false)
	b := mustAdd(t, s, 0, "B", false)
	c := mustAdd(t, s, 0, "C", false)
	d := mustAdd(t, s, 0, "D", false)
	s.Link(a.ID, b.ID)
	s.Link(b.ID, c.ID) // links are followed further: A hears C
	for i, ch := range []uint32{a.ID, a.ID, b.ID, c.ID, d.ID} {
		s.AddUser(&User{Session: uint32(i + 1), Name: string(rune('a' + i)), Channel: ch})
	}
	s.Users[2].SelfDeaf = true
	got := s.Listeners(1)
	slices.Sort(got)
	if !slices.Equal(got, []uint32{3, 4}) {
		t.Fatalf("listeners %v", got)
	}
	for _, mute := range []func(u *User){
		func(u *User) { u.SelfMute = true },
		func(u *User) { u.Mute = true },
		func(u *User) { u.Suppress = true },
	} {
		u := *s.Users[1]
		mute(s.Users[1])
		if len(s.Listeners(1)) != 0 {
			t.Fatal("a muted user was heard")
		}
		*s.Users[1] = u
	}
}

func TestTemporaryChannelsHaveNoSubchannels(t *testing.T) {
	s := newTestState(t)
	tmp := mustAdd(t, s, 0, "Tmp", true)
	if _, err := s.AddChannel(tmp.ID, "Inside", false); denyType(err) != pb.PermissionDenied_TemporaryChannel {
		t.Fatalf("made a channel in a temporary channel: %v", err)
	}
	a := mustAdd(t, s, 0, "A", false)
	if err := s.EditChannel(a.ID, ChannelEdit{Parent: &tmp.ID}); denyType(err) != pb.PermissionDenied_TemporaryChannel {
		t.Fatalf("moved a channel into a temporary channel: %v", err)
	}
}

func TestOrphansAreMovedBelowRoot(t *testing.T) {
	s := NewState("Root", []Channel{{ID: 5, Parent: 4, Name: "Orphan"}, {ID: 6, Parent: 5, Name: "Child"}})
	if s.Channels[5].Parent != 0 || s.Channels[6].Parent != 5 {
		t.Fatalf("orphan at %d", s.Channels[5].Parent)
	}
	if len(s.Tree()) != 3 {
		t.Fatal("orphan not in the tree")
	}
}

func TestChannelNamesMayUseAnyLetters(t *testing.T) {
	s := newTestState(t)
	for _, name := range []string{"Küche", "Канал", "部屋 2"} {
		if _, err := s.AddChannel(0, name, false); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRootCannotBeRenamed(t *testing.T) {
	s := newTestState(t)
	name := "Other"
	if err := s.EditChannel(0, ChannelEdit{Name: &name}); err == nil {
		t.Fatal("renamed root")
	}
	desc := "about the server"
	if err := s.EditChannel(0, ChannelEdit{Description: &desc}); err != nil {
		t.Fatal(err)
	}
}

func TestSavedRootKeepsTheConfiguredName(t *testing.T) {
	s := NewState("Kallisti", []Channel{{ID: 0, Name: "Old", Description: "hello"}})
	if s.Channels[0].Name != "Kallisti" || s.Channels[0].Description != "hello" {
		t.Fatalf("root %+v", s.Channels[0])
	}
}

func TestPlainText(t *testing.T) {
	for in, want := range map[string]string{
		"<b>hi</b> all": "hi all",
		"a < b > c":     "a < b > c",
		"x <img src=a":  "x",
		"<!-- c -->ok":  "ok",
		"  spaced  ":    "spaced",
	} {
		if got := plainText(in); got != want {
			t.Errorf("plainText(%q) = %q, want %q", in, got, want)
		}
	}
}
