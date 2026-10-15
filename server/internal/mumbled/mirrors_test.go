package mumbled

import (
	"slices"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// mirrorTree is the text channel parent and its children's names, in
// order, as the server state has them.
func mirrorTree(t *testing.T, s *Server) (parent *Channel, names []string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.state.Channels {
		if c.Parent == 0 && c.ID != 0 && c.Name == "Text channels" {
			parent = c
		}
	}
	if parent == nil {
		t.Fatal("no text channel parent")
	}
	var kids []*Channel
	for _, c := range s.state.Channels {
		if c.Parent == parent.ID && c.ID != parent.ID {
			kids = append(kids, c)
		}
	}
	slices.SortFunc(kids, func(a, b *Channel) int { return int(a.Position - b.Position) })
	for _, c := range kids {
		names = append(names, c.Name)
	}
	return parent, names
}

func TestSyncTextChannels(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	ids, err := s.SyncTextChannels([]TextMirror{{Name: "general"}, {Name: "random"}})
	if err != nil || len(ids) != 2 {
		t.Fatalf("sync: %v %v", ids, err)
	}
	if _, names := mirrorTree(t, s); !slices.Equal(names, []string{"#general", "#random"}) {
		t.Fatalf("names %v", names)
	}
	// Rename and reorder; then drop one.
	again, _ := s.SyncTextChannels([]TextMirror{{Channel: ids[1], Name: "games"}, {Channel: ids[0], Name: "general"}})
	if !slices.Equal(again, []uint32{ids[1], ids[0]}) {
		t.Fatalf("ids %v, want %v", again, []uint32{ids[1], ids[0]})
	}
	if _, names := mirrorTree(t, s); !slices.Equal(names, []string{"#games", "#general"}) {
		t.Fatalf("after rename: %v", names)
	}
	s.SyncTextChannels([]TextMirror{{Channel: ids[0], Name: "general"}})
	if _, names := mirrorTree(t, s); !slices.Equal(names, []string{"#general"}) {
		t.Fatalf("after delete: %v", names)
	}
	// A restart keeps the parent and the mirrors (no duplicates).
	s.Close()
	s2, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	ids2, err := s2.SyncTextChannels([]TextMirror{{Channel: ids[0], Name: "general"}})
	if err != nil || !slices.Equal(ids2, []uint32{ids[0]}) {
		t.Fatalf("after restart: %v %v", ids2, err)
	}
	parents := 0
	s2.mu.Lock()
	for _, ch := range s2.state.Channels {
		if ch.Name == "Text channels" {
			parents++
		}
	}
	s2.mu.Unlock()
	if parents != 1 {
		t.Fatalf("%d parents", parents)
	}
}

func TestSyncRecreatesMissingMirrors(t *testing.T) {
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: testDB(t)}, nil)
	ids, _ := s.SyncTextChannels([]TextMirror{{Name: "general"}})
	parent, _ := mirrorTree(t, s)
	s.mu.Lock()
	s.state.RemoveChannel(parent.ID) // SuperUser may delete it in a Mumble client
	s.mu.Unlock()
	again, err := s.SyncTextChannels([]TextMirror{{Channel: ids[0], Name: "general"}})
	if err != nil || again[0] == ids[0] {
		t.Fatalf("again %v %v", again, err)
	}
	if _, names := mirrorTree(t, s); !slices.Equal(names, []string{"#general"}) {
		t.Fatalf("names %v", names)
	}
}

func TestNobodySpeaksOrBuildsInMirrors(t *testing.T) {
	s, addr := rolesServer(t)
	ids, _ := s.SyncTextChannels([]TextMirror{{Name: "general"}})
	alice, aliceID := registered(t, addr, "alice")
	su := superUser(t, addr)
	s.SetModerator(su.session, aliceID, true)
	alice.move(alice.session, ids[0])
	alice.waitForMove(alice.session, ids[0])
	s.mu.Lock()
	u := s.state.Users[alice.session]
	perms := s.state.Permissions(u, ids[0])
	s.mu.Unlock()
	if perms&(Speak|Write|MakeChannel|MakeTempChannel) != 0 || perms&(Enter|TextMessage) != Enter|TextMessage {
		t.Fatalf("moderator's permissions in a mirror: %#x", perms)
	}
	if !u.Suppress {
		t.Fatal("not suppressed")
	}
}

func TestUserMovesAreReported(t *testing.T) {
	var mu sync.Mutex
	var got [][3]uint32
	cfg := Config{OnUserMoved: func(session, from, to uint32) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, [3]uint32{session, from, to})
	}}
	s, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	ids, _ := s.SyncTextChannels([]TextMirror{{Name: "general"}})
	c := dial(t, addr, "mover")
	c.move(c.session, ids[0])
	c.waitForMove(c.session, ids[0])
	c.conn.Close()
	want := [][3]uint32{{c.session, NoChannel, 0}, {c.session, 0, ids[0]}, {c.session, ids[0], NoChannel}}
	eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Equal(got, want)
	})
}

func TestServerTextsGoToOneUser(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	c := dial(t, addr, "reader")
	s.SendServerText(c.session, "<b>bob</b>: hi")
	var m pb.TextMessage
	proto.Unmarshal(c.expect(mumble.TypeTextMessage), &m)
	if m.Actor != nil || m.GetMessage() != "<b>bob</b>: hi" || !slices.Equal(m.Session, []uint32{c.session}) {
		t.Fatalf("text %v", &m)
	}
}

func TestTreeTextsSkipMirrors(t *testing.T) {
	var mu sync.Mutex
	var got []uint32
	cfg := Config{OnChannelText: func(_ uint32, _ string, _ bool, channel uint32, _ string) {
		mu.Lock()
		got = append(got, channel)
		mu.Unlock()
	}}
	s, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	ids, _ := s.SyncTextChannels([]TextMirror{{Name: "general"}})
	alice := dial(t, addr, "alice")
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{TreeId: []uint32{0}, Message: proto.String("to all")})
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{ids[0]}, Message: proto.String("to #general")})
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 2 })
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(got, []uint32{0, ids[0]}) {
		t.Fatalf("hook got %v", got)
	}
}

func TestNobodyWritesInTheMirrorsParent(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, nil)
	s.SyncTextChannels([]TextMirror{{Name: "general"}})
	parent, _ := mirrorTree(t, s)
	c := dial(t, addr, "guest")
	s.mu.Lock()
	u := s.state.Users[c.session]
	here, below := s.state.Has(u, parent.ID, TextMessage), s.state.Has(u, parent.ID+1, TextMessage)
	s.mu.Unlock()
	if here || !below {
		t.Fatalf("TextMessage in the parent %v, in a mirror %v", here, below)
	}
}

func TestSyncAdoptsAnUnsavedParent(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	s.SyncTextChannels([]TextMirror{{Name: "general"}})
	db.SetTextParent(0) // as if saving the parent's id had failed
	if _, err := s.SyncTextChannels([]TextMirror{{Name: "general"}}); err != nil {
		t.Fatal(err)
	}
	mirrorTree(t, s)
}
