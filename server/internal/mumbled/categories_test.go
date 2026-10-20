package mumbled

import (
	"slices"
	"testing"
	"time"
)

// childNames: a channel's children, by position.
func childNames(s *Server, parent uint32) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kids []*Channel
	for _, c := range s.state.Channels {
		if c.Parent == parent && c.ID != 0 {
			kids = append(kids, c)
		}
	}
	slices.SortFunc(kids, func(a, b *Channel) int { return int(a.Position - b.Position) })
	out := []string{}
	for _, c := range kids {
		out = append(out, c.Name)
	}
	return out
}

func parentOf(s *Server, id uint32) uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Channels[id].Parent
}

func TestSyncTextTreeWithCategories(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	cats, ids, err := s.SyncTextTree([]TextCategory{{Name: "Games"}, {Name: "Music"}},
		[]TextMirror{{Name: "general"}, {Name: "minecraft", Category: 1}, {Name: "valorant", Category: 1}})
	if err != nil || len(cats) != 2 || len(ids) != 3 {
		t.Fatalf("sync: %v %v %v", cats, ids, err)
	}
	parent, names := mirrorTree(t, s)
	if !slices.Equal(names, []string{"#general", "Games", "Music"}) {
		t.Fatalf("parent's children %v", names)
	}
	if got := childNames(s, cats[0]); !slices.Equal(got, []string{"#minecraft", "#valorant"}) {
		t.Fatalf("Games has %v", got)
	}
	// Moves keep the mirrors: general into Music, minecraft out; Games
	// renamed.
	cats2, ids2, _ := s.SyncTextTree([]TextCategory{{Channel: cats[0], Name: "Gaming"}, {Channel: cats[1], Name: "Music"}},
		[]TextMirror{{Channel: ids[1], Name: "minecraft"}, {Channel: ids[0], Name: "general", Category: 2}, {Channel: ids[2], Name: "valorant", Category: 1}})
	if !slices.Equal(cats2, cats) || !slices.Equal(ids2, []uint32{ids[1], ids[0], ids[2]}) {
		t.Fatalf("ids changed: %v %v", cats2, ids2)
	}
	if _, names := mirrorTree(t, s); !slices.Equal(names, []string{"#minecraft", "Gaming", "Music"}) {
		t.Fatalf("after moves %v", names)
	}
	if parentOf(s, ids[0]) != cats[1] || parentOf(s, ids[1]) != parent.ID {
		t.Fatal("not moved")
	}
	// A deleted category's channels stay (uncategorized).
	_, ids3, _ := s.SyncTextTree([]TextCategory{{Channel: cats[0], Name: "Gaming"}},
		[]TextMirror{{Channel: ids[1], Name: "minecraft"}, {Channel: ids[0], Name: "general"}, {Channel: ids[2], Name: "valorant", Category: 1}})
	if !slices.Equal(ids3, []uint32{ids[1], ids[0], ids[2]}) {
		t.Fatalf("ids %v", ids3)
	}
	if _, names := mirrorTree(t, s); !slices.Equal(names, []string{"#minecraft", "#general", "Gaming"}) {
		t.Fatalf("after delete %v", names)
	}
	// A restart keeps the tree.
	s.Close()
	s2, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	cats4, ids4, err := s2.SyncTextTree([]TextCategory{{Channel: cats[0], Name: "Gaming"}},
		[]TextMirror{{Channel: ids[1], Name: "minecraft"}, {Channel: ids[0], Name: "general"}, {Channel: ids[2], Name: "valorant", Category: 1}})
	if err != nil || cats4[0] != cats[0] || !slices.Equal(ids4, ids3) {
		t.Fatalf("after restart: %v %v %v", cats4, ids4, err)
	}
}

// A category is a channel moderators' channel: its moderators moderate
// its text channels, but not private ones they are not members of.
func TestCategoryModeratorsModerateItsChannels(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	alice, aliceID := registered(t, addr, "alice")
	bob, bobID := registered(t, addr, "bob")
	cats, ids, err := s.SyncTextTree([]TextCategory{{Name: "Games"}},
		[]TextMirror{{Name: "general"}, {Name: "minecraft", Category: 1}, {Name: "secret", Category: 1, Private: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetChannelModerator(su.session, aliceID, cats[0], true); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		channel uint32
		want    bool
	}{{cats[0], true}, {ids[1], true}, {ids[0], false}, {ids[2], false}} {
		if got := s.ModeratesIn(alice.session, c.channel); got != c.want {
			t.Errorf("alice moderates %d: %v", c.channel, got)
		}
	}
	if s.ModeratesIn(bob.session, ids[1]) {
		t.Fatal("bob moderates")
	}
	// As a member she moderates the private one too; bob, a member, only
	// reads it.
	if err := s.SetPrivate(su.session, ids[2], true, []uint32{aliceID, bobID}); err != nil {
		t.Fatal(err)
	}
	if !s.ModeratesIn(alice.session, ids[2]) || s.ModeratesIn(bob.session, ids[2]) || !s.Sees(bob.session, ids[2]) {
		t.Fatal("private channel in a category")
	}
	// The moderators' entries stay on a later sync.
	s.SyncTextTree([]TextCategory{{Channel: cats[0], Name: "Games"}},
		[]TextMirror{{Channel: ids[0], Name: "general"}, {Channel: ids[1], Name: "minecraft", Category: 1}, {Channel: ids[2], Name: "secret", Category: 1}})
	if !s.ModeratesIn(alice.session, ids[1]) || len(s.Private()) != 1 {
		t.Fatal("lost after a sync")
	}
	// Channel time-outs in a category hold in its channels.
	if err := s.SetChannelTimeout(alice.session, bob.session, cats[0], time.Minute); err != nil {
		t.Fatal(err)
	}
	if s.MayWrite(bob.session, ids[1]) || !s.MayWrite(bob.session, ids[0]) {
		t.Fatal("a category time-out")
	}
}

// Nobody writes in a category or speaks in its mirrors; categories and
// their mirrors are as locked as other mirrors.
func TestCategoriesAreLockedLikeMirrors(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	alice, _ := registered(t, addr, "alice")
	cats, ids, _ := s.SyncTextTree([]TextCategory{{Name: "Games"}}, []TextMirror{{Name: "minecraft", Category: 1}})
	if s.MayWrite(alice.session, cats[0]) || !s.MayWrite(alice.session, ids[0]) || !s.Sees(alice.session, cats[0]) {
		t.Fatal("category texts")
	}
	s.mu.Lock()
	u := s.state.Users[alice.session]
	speaks := s.state.Has(u, ids[0], Speak) || s.state.Has(u, cats[0], Speak) || s.state.Has(u, cats[0], MakeChannel)
	locked := s.isMirrorLocked(cats[0]) && s.isMirrorLocked(ids[0])
	s.mu.Unlock()
	if speaks || !locked {
		t.Fatalf("speaks %v, locked %v", speaks, locked)
	}
	if err := s.SetListenOnly(su.session, ids[0], true, nil); err == nil {
		t.Fatal("a listen-only mirror")
	}
}
