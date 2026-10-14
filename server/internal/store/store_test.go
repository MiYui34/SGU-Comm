package store

import (
	"testing"
	"time"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func names(cs []Channel) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func TestEnsureChannelsIsIdempotentAndOrdered(t *testing.T) {
	s := open(t)
	if err := s.EnsureChannels([]string{"general", "links"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureChannels([]string{"links", "general", "memes"}); err != nil {
		t.Fatal(err)
	}
	cs, err := s.Channels()
	if err != nil {
		t.Fatal(err)
	}
	// The latest configuration decides the order. Removed names stay hidden.
	if got := names(cs); len(got) != 3 || got[0] != "links" || got[1] != "general" || got[2] != "memes" {
		t.Fatalf("got %v", got)
	}
	s.EnsureChannels([]string{"general"})
	cs, _ = s.Channels()
	if got := names(cs); len(got) != 1 || got[0] != "general" {
		t.Fatalf("after removal: got %v", got)
	}
}

func TestMessagesAndHistory(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general", "links"})
	cs, _ := s.Channels()
	general, links := cs[0].ID, cs[1].ID
	base := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	var ids []int64
	for i := 0; i < 5; i++ {
		m, err := s.AddMessage(general, "alice", string(rune('a'+i)), base.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	s.AddMessage(links, "bob", "elsewhere", base)

	newest, err := s.History(general, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(newest) != 2 || newest[0].Text != "d" || newest[1].Text != "e" {
		t.Fatalf("newest page: %+v", newest)
	}
	older, _ := s.History(general, newest[0].ID, 10)
	if len(older) != 3 || older[0].Text != "a" || older[2].Text != "c" {
		t.Fatalf("older page: %+v", older)
	}
	if !older[0].CreatedAt.Equal(base) || older[0].Author != "alice" || older[0].ChannelID != general {
		t.Fatalf("fields: %+v", older[0])
	}
}

func TestAddMessageToUnknownChannelFails(t *testing.T) {
	s := open(t)
	if _, err := s.AddMessage(999, "alice", "x", time.Now()); err == nil {
		t.Fatal("expected an error")
	}
}
