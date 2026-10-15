package sidecar

import (
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// joinAs is ready like readyAs, with registration.
func joinAs(t *testing.T, h *Hub, id mumble.Identity) *Conn {
	t.Helper()
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(id)
	if m := next(t, c); m["type"] != "ready" {
		t.Fatalf("got %v", m)
	}
	return c
}

// say posts to the first text channel and returns the message id.
func say(t *testing.T, h *Hub, c *Conn, text string) int64 {
	t.Helper()
	ch := strconv.FormatInt(h.channels[0].ID, 10)
	c.Handle([]byte(`{"type":"send","channel":` + ch + `,"text":"` + text + `"}`))
	m := next(t, c)
	if m["type"] != "message" {
		t.Fatalf("send: %v", m)
	}
	return int64(m["message"].(map[string]any)["id"].(float64))
}

func TestEditAndDeleteOwnMessages(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	id := strconv.FormatInt(say(t, h, alice, "helo"), 10)
	next(t, bob)
	alice.Handle([]byte(`{"type":"edit-message","message":` + id + `,"text":"hello"}`))
	for _, c := range []*Conn{alice, bob} {
		m := next(t, c)
		msg := m["message"].(map[string]any)
		if m["type"] != "message-edited" || msg["text"] != "hello" || msg["edited"] == nil {
			t.Fatalf("got %v", m)
		}
	}
	// Bob may not touch it.
	bob.Handle([]byte(`{"type":"edit-message","message":` + id + `,"text":"pwned"}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("bob edited: %v", m)
	}
	bob.Handle([]byte(`{"type":"delete-message","message":` + id + `}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("bob deleted: %v", m)
	}
	alice.Handle([]byte(`{"type":"delete-message","message":` + id + `}`))
	for _, c := range []*Conn{alice, bob} {
		if m := next(t, c); m["type"] != "message-deleted" || m["id"] != float64(must(strconv.Atoi(id))) {
			t.Fatalf("got %v", m)
		}
	}
}

func must(n int, _ error) int { return n }

func TestGuestsEditOnlyWhatTheyWroteThisVisit(t *testing.T) {
	h := newHub(t, "general")
	first := joinAs(t, h, mumble.Identity{Session: 3, Name: "guest"})
	id := strconv.FormatInt(say(t, h, first, "mine"), 10)
	first.Handle([]byte(`{"type":"edit-message","message":` + id + `,"text":"still mine"}`))
	if m := next(t, first); m["type"] != "message-edited" {
		t.Fatalf("own edit: %v", m)
	}
	// Another guest with that name later (names of guests are free).
	first.Close()
	later := joinAs(t, h, mumble.Identity{Session: 4, Name: "guest"})
	later.Handle([]byte(`{"type":"delete-message","message":` + id + `}`))
	if m := next(t, later); m["type"] != "error" {
		t.Fatalf("other guest deleted: %v", m)
	}
}

func TestModeratorsDeleteAnyMessage(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "") // session 1 may manage
	var removed []string
	h.OnUploadDeleted = func(key string) { removed = append(removed, key) }
	mod := joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	h.store.AddUpload(store.Upload{Key: "spamkey", Name: "spam.png", Type: "image/png", Size: 1, Author: "bob"}, clock)
	ch := strconv.FormatInt(h.channels[0].ID, 10)
	bob.Handle([]byte(`{"type":"send","channel":` + ch + `,"text":"spam","upload":"spamkey"}`))
	sent := next(t, bob)
	id := strconv.FormatInt(int64(sent["message"].(map[string]any)["id"].(float64)), 10)
	next(t, mod)
	mod.Handle([]byte(`{"type":"edit-message","message":` + id + `,"text":"x"}`))
	if m := next(t, mod); m["type"] != "error" {
		t.Fatalf("moderator edited: %v", m)
	}
	mod.Handle([]byte(`{"type":"delete-message","message":` + id + `}`))
	if m := next(t, bob); m["type"] != "message-deleted" {
		t.Fatalf("bob got %v", m)
	}
	if len(removed) != 1 || removed[0] != "spamkey" {
		t.Fatalf("files removed: %v", removed)
	}
}

func TestAReusedGuestSessionOwnsNothing(t *testing.T) {
	h := newHub(t, "general")
	first := joinAs(t, h, mumble.Identity{Session: 5, Name: "guest"})
	id := strconv.FormatInt(say(t, h, first, "mine"), 10)
	first.Close()
	// Murmur gives the session number to the next guest.
	next5 := joinAs(t, h, mumble.Identity{Session: 5, Name: "guest"})
	next5.Handle([]byte(`{"type":"delete-message","message":` + id + `}`))
	if m := next(t, next5); m["type"] != "error" {
		t.Fatalf("reused session deleted: %v", m)
	}
}

func TestEditingAVoiceChatNeedsWrite(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	// Session 2 may not write to (or read) channel 9 in the fake.
	chat, _ := h.store.VoiceChat(9)
	m, _ := h.store.AddMessage(store.NewMessage{ChannelID: chat, Author: "bob", Verified: true, Text: "x", At: clock})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	bob.Handle([]byte(`{"type":"edit-message","message":` + strconv.FormatInt(m.ID, 10) + `,"text":"y"}`))
	if got := next(t, bob); got["type"] != "error" {
		t.Fatalf("edited without write: %v", got)
	}
}

func TestReplies(t *testing.T) {
	h := newHub(t, "general", "random")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	orig := say(t, h, alice, "question?")
	next(t, bob)
	general := strconv.FormatInt(h.channels[0].ID, 10)
	bob.Handle([]byte(`{"type":"send","channel":` + general + `,"text":"answer","reply":` + strconv.FormatInt(orig, 10) + `}`))
	m := next(t, bob)
	q, ok := m["message"].(map[string]any)["reply"].(map[string]any)
	if !ok || q["id"] != float64(orig) || q["author"] != "alice" || q["text"] != "question?" {
		t.Fatalf("reply %v", m)
	}
	random := strconv.FormatInt(h.channels[1].ID, 10)
	bob.Handle([]byte(`{"type":"send","channel":` + random + `,"text":"x","reply":` + strconv.FormatInt(orig, 10) + `}`))
	next(t, alice) // the answer above
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("cross-channel reply: %v", m)
	}
	// Alice comes back later: the reply counts as a mention of her.
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 3, Name: "alice", Registered: true})
	ready := next(t, c)
	if e := ready["summary"].([]any)[0].(map[string]any); e["mentions"] != float64(1) {
		t.Fatalf("summary %v", ready["summary"])
	}
}
