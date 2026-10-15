package store

import (
	"errors"
	"slices"
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

func channelNames(cs []Channel) []string {
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
	if got := channelNames(cs); len(got) != 3 || got[0] != "links" || got[1] != "general" || got[2] != "memes" {
		t.Fatalf("got %v", got)
	}
	s.EnsureChannels([]string{"general"})
	cs, _ = s.Channels()
	if got := channelNames(cs); len(got) != 1 || got[0] != "general" {
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
		m, err := s.AddMessage(NewMessage{ChannelID: general, Author: "alice", Text: string(rune('a' + i)), At: base.Add(time.Duration(i) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	s.AddMessage(NewMessage{ChannelID: links, Author: "bob", Text: "elsewhere", At: base})

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
	if _, err := s.AddMessage(NewMessage{ChannelID: 999, Author: "alice", Text: "x", At: time.Now()}); err == nil {
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
	m, err := s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Text: "look", At: at, UploadID: u.ID})
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
	if _, err := s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Text: "again", At: at, UploadID: u.ID}); err == nil {
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
	if _, err := s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Text: "hi", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUnusedUploads(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	old := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	stale, _ := s.AddUpload(Upload{Key: "stale", Name: "a", Type: "x", Size: 1, Author: "a"}, old)
	attached, _ := s.AddUpload(Upload{Key: "attached", Name: "b", Type: "x", Size: 1, Author: "a"}, old)
	s.AddMessage(NewMessage{ChannelID: 1, Author: "a", Text: "", At: old, UploadID: attached.ID})
	s.AddUpload(Upload{Key: "fresh", Name: "c", Type: "x", Size: 1, Author: "a"}, old.Add(time.Hour))
	keys, err := s.DeleteUnusedUploads(old.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "stale" {
		t.Fatalf("deleted %v", keys)
	}
	if _, err := s.UploadByKey("stale"); err == nil {
		t.Fatal("stale upload still there")
	}
	_ = stale
	for _, k := range []string{"attached", "fresh"} {
		if _, err := s.UploadByKey(k); err != nil {
			t.Fatalf("%s was deleted", k)
		}
	}
}

func TestVerifiedAuthors(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Verified: true, Text: "a", At: time.Now()})
	s.AddMessage(NewMessage{ChannelID: 1, Author: "bob", Text: "b", At: time.Now()})
	hist, _ := s.History(1, 0, 10)
	if !hist[0].Verified || hist[1].Verified {
		t.Fatalf("got %+v", hist)
	}
}

func TestUploadedBytesSince(t *testing.T) {
	s := open(t)
	day := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s.AddUpload(Upload{Key: "a", Name: "a", Type: "x", Size: 100, Author: "alice"}, day.Add(-48*time.Hour))
	s.AddUpload(Upload{Key: "b", Name: "b", Type: "x", Size: 200, Author: "alice"}, day)
	s.AddUpload(Upload{Key: "c", Name: "c", Type: "x", Size: 300, Author: "alice"}, day.Add(time.Hour))
	s.AddUpload(Upload{Key: "d", Name: "d", Type: "x", Size: 999, Author: "bob"}, day)
	n, err := s.UploadedBytesSince("alice", day.Add(-24*time.Hour))
	if err != nil || n != 500 {
		t.Fatalf("got %d %v, want 500", n, err)
	}
}

func TestAccounts(t *testing.T) {
	s := open(t)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	a, err := s.CreateAccount(Account{Name: "Alice", PasswordHash: "h", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now)
	if err != nil || a.ID == 0 {
		t.Fatalf("create: %+v %v", a, err)
	}
	if _, err := s.CreateAccount(Account{Name: "alice", PasswordHash: "h2", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now); err != ErrNameTaken {
		t.Fatalf("duplicate (other case): got %v, want ErrNameTaken", err)
	}
	got, err := s.AccountByName("ALICE")
	if err != nil || got.ID != a.ID || got.Name != "Alice" || string(got.CertPEM) != "c" || got.PasswordHash != "h" {
		t.Fatalf("by name: %+v %v", got, err)
	}
	if _, err := s.AccountByName("bob"); err != ErrNotFound {
		t.Fatalf("missing: got %v", err)
	}
}

func TestSessions(t *testing.T) {
	s := open(t)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	a, err := s.CreateAccount(Account{Name: "alice", PasswordHash: "h", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession("hash1", a.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := s.AccountBySession("hash1", now)
	if err != nil || got.ID != a.ID {
		t.Fatalf("session: %+v %v", got, err)
	}
	if _, err := s.AccountBySession("hash1", now.Add(2*time.Hour)); err != ErrNotFound {
		t.Fatalf("expired session: got %v", err)
	}
	if err := s.DeleteSession("hash1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccountBySession("hash1", now); err != ErrNotFound {
		t.Fatalf("deleted session: got %v", err)
	}
}

func TestAccountNamesCompareByKey(t *testing.T) {
	s := open(t)
	now := time.Now()
	if _, err := s.CreateAccount(Account{Name: "Émile", PasswordHash: "h", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now); err != nil {
		t.Fatal(err)
	}
	// Lower case and decomposed: the same name.
	if _, err := s.CreateAccount(Account{Name: "émile", PasswordHash: "h", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now); err != ErrNameTaken {
		t.Fatalf("got %v, want ErrNameTaken", err)
	}
	if a, err := s.AccountByName(" ÉMILE "); err != nil || a.Name != "Émile" {
		t.Fatalf("by name: %+v %v", a, err)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	s := open(t)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	a, _ := s.CreateAccount(Account{Name: "alice", PasswordHash: "h", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now)
	s.CreateSession("old", a.ID, now.Add(-time.Minute))
	s.CreateSession("new", a.ID, now.Add(time.Hour))
	if n, err := s.DeleteExpiredSessions(now); err != nil || n != 1 {
		t.Fatalf("deleted %d %v", n, err)
	}
	if _, err := s.AccountBySession("new", now); err != nil {
		t.Fatal("valid session deleted")
	}
}

func TestPasswordAndSessionsOfAnAccount(t *testing.T) {
	s := open(t)
	now := time.Now()
	a, _ := s.CreateAccount(Account{Name: "alice", PasswordHash: "h", CertPEM: []byte("c"), KeyPEM: []byte("k")}, now)
	s.CreateSession("one", a.ID, now.Add(time.Hour))
	s.CreateSession("two", a.ID, now.Add(time.Hour))
	if err := s.SetPassword(a.ID, "h2"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.AccountByName("alice"); got.PasswordHash != "h2" {
		t.Fatalf("hash %q", got.PasswordHash)
	}
	if err := s.DeleteSessionsExcept(a.ID, "two"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccountBySession("one", now); err != ErrNotFound {
		t.Error("session one still valid")
	}
	if _, err := s.AccountBySession("two", now); err != nil {
		t.Error("session two was deleted")
	}
	if err := s.DeleteSessionsExcept(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccountBySession("two", now); err != ErrNotFound {
		t.Error("session two still valid after deleting all")
	}
}

func TestTheDatabaseIsShared(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.DB().Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestVoiceChannelChats(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.EnsureChannels([]string{"general"})
	id, err := s.VoiceChat(5)
	if err != nil || id == 0 {
		t.Fatalf("id %d err %v", id, err)
	}
	again, _ := s.VoiceChat(5)
	other, _ := s.VoiceChat(6)
	if again != id || other == id {
		t.Fatalf("ids %d %d %d", id, again, other)
	}
	if voice, ok, _ := s.VoiceOf(id); !ok || voice != 5 {
		t.Fatalf("voice of %d: %d %v", id, voice, ok)
	}
	if _, ok, _ := s.VoiceOf(1); ok {
		t.Fatal("a text channel is a voice chat")
	}
	// Voice chats are not text channels, also after a restart.
	s.EnsureChannels([]string{"general"})
	chans, _ := s.Channels()
	if len(chans) != 1 || chans[0].Name != "general" {
		t.Fatalf("channels %+v", chans)
	}
	// Messages work as in text channels.
	if _, err := s.AddMessage(NewMessage{ChannelID: id, Author: "alice", Text: "hi", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if msgs, _ := s.History(id, 0, 10); len(msgs) != 1 {
		t.Fatalf("history %+v", msgs)
	}
}

func TestManagingTextChannels(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// The configured channels only seed a new server.
	if err := s.SeedChannels([]string{"general", "links"}); err != nil {
		t.Fatal(err)
	}
	s.SeedChannels([]string{"other"})
	names := func() []string {
		chans, _ := s.Channels()
		out := []string{}
		for _, c := range chans {
			out = append(out, c.Name)
		}
		return out
	}
	if got := names(); !slices.Equal(got, []string{"general", "links"}) {
		t.Fatalf("seeded %v", got)
	}
	games, err := s.CreateChannel("games")
	if err != nil || games.Name != "games" {
		t.Fatalf("create %+v %v", games, err)
	}
	if _, err := s.CreateChannel("games"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := s.RenameChannel(games.ID, "gaming"); err != nil {
		t.Fatal(err)
	}
	chans, _ := s.Channels()
	if err := s.SetPositions([]int64{games.ID, chans[0].ID, chans[1].ID}); err != nil {
		t.Fatal(err)
	}
	if got := names(); !slices.Equal(got, []string{"gaming", "general", "links"}) {
		t.Fatalf("after moves %v", got)
	}
	if err := s.HideChannel(games.ID); err != nil {
		t.Fatal(err)
	}
	if got := names(); !slices.Equal(got, []string{"general", "links"}) {
		t.Fatalf("after delete %v", got)
	}
	if back, err := s.CreateChannel("gaming"); err != nil || back.ID != games.ID {
		t.Fatalf("made again: %+v %v (want the old channel back)", back, err)
	}
	// Voice channel chats cannot be renamed or hidden as text channels.
	voice, _ := s.VoiceChat(3)
	if err := s.RenameChannel(voice, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("renamed a voice chat: %v", err)
	}
}

func TestTextChannelNamesCannotLookLikeVoiceChats(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateChannel("voice:7"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("create: %v", err)
	}
	if err := s.EnsureChannels([]string{"voice:7"}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("ensure: %v", err)
	}
	// Configured channels never show a voice chat.
	id, _ := s.VoiceChat(7)
	s.EnsureChannels([]string{"general"})
	if chans, _ := s.Channels(); len(chans) != 1 {
		t.Fatalf("channels %+v", chans)
	}
	if voice, ok, _ := s.VoiceOf(id); !ok || voice != 7 {
		t.Fatal("voice chat lost")
	}
}

func TestTextChannelsRememberTheirMumbleChannel(t *testing.T) {
	s := open(t)
	if err := s.EnsureChannels([]string{"general"}); err != nil {
		t.Fatal(err)
	}
	chans, _ := s.Channels()
	if chans[0].Mumble != 0 {
		t.Fatalf("new channel: %+v", chans[0])
	}
	if err := s.SetMumbleChannel(chans[0].ID, 42); err != nil {
		t.Fatal(err)
	}
	chans, _ = s.Channels()
	if chans[0].Mumble != 42 {
		t.Fatalf("after set: %+v", chans[0])
	}
}
