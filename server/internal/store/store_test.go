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

func TestUploadsAndAttachments(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	at := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	u, err := s.AddUpload(Upload{Key: "k1", Name: "cat.png", Type: "image/png", Size: 1234, Author: "alice"}, at)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UploadByKey("k1")
	if err != nil || got.ID != u.ID || got.Name != "cat.png" || got.Author != "alice" || got.Used {
		t.Fatalf("got %+v %v", got, err)
	}
	m, err := s.AddMessageWithUpload(1, "alice", "look", at, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Upload == nil || m.Upload.Name != "cat.png" {
		t.Fatalf("message upload: %+v", m.Upload)
	}
	got, _ = s.UploadByKey("k1")
	if !got.Used {
		t.Fatal("upload not marked as used")
	}
	// An upload can be attached only once.
	if _, err := s.AddMessageWithUpload(1, "alice", "again", at, u.ID); err == nil {
		t.Fatal("expected an error for a used upload")
	}
	hist, _ := s.History(1, 0, 10)
	if len(hist) != 1 || hist[0].Upload == nil || hist[0].Upload.Key != "k1" || hist[0].Upload.Size != 1234 {
		t.Fatalf("history: %+v", hist)
	}
	if _, err := s.UploadByKey("nope"); err == nil {
		t.Fatal("expected an error for a missing upload")
	}
}

func TestMigratesAnOldDatabase(t *testing.T) {
	path := t.TempDir() + "/old.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the first schema: no upload column.
	if _, err := s.db.Exec(`ALTER TABLE messages DROP COLUMN upload_id`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	s.EnsureChannels([]string{"general"})
	if _, err := s.AddMessage(1, "alice", "hi", time.Now()); err != nil {
		t.Fatal(err)
	}
}
