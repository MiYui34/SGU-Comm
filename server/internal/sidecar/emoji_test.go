package sidecar

import (
	"fmt"
	"testing"
	"time"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func addEmoji(t *testing.T, h *Hub, name string) store.Emoji {
	t.Helper()
	e, err := h.store.AddEmoji(store.Emoji{Name: name, Type: "image/png", Data: []byte("\x89PNG"), Author: "admin"}, clock)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func emojiNames(t *testing.T, list any) []string {
	t.Helper()
	var names []string
	for _, e := range list.([]any) {
		names = append(names, e.(map[string]any)["name"].(string))
	}
	return names
}

func TestReadyAndChangesSendTheCustomEmoji(t *testing.T) {
	h := newHub(t, "general")
	e := addEmoji(t, h, "party")
	h.EmojiChanged() // as after an upload
	c := h.Connect()
	defer c.Close()
	c.SetIdentity(mumble.Identity{Session: 4, Name: "alice"})
	m := next(t, c)
	list := m["emoji"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["url"] != fmt.Sprintf("/api/emoji/%d", e.ID) {
		t.Fatalf("ready %v", m)
	}
	addEmoji(t, h, "cat")
	h.EmojiChanged()
	m = next(t, c)
	if names := emojiNames(t, m["emoji"]); m["type"] != "emoji" || fmt.Sprint(names) != "[cat party]" {
		t.Fatalf("change %v", m)
	}
}

func TestReadyHasAnEmptyEmojiList(t *testing.T) {
	h := newHub(t, "general")
	c := h.Connect()
	defer c.Close()
	c.SetIdentity(mumble.Identity{Session: 4, Name: "alice"})
	if m := next(t, c); m["emoji"] == nil || len(m["emoji"].([]any)) != 0 {
		t.Fatalf("got %v", m)
	}
}

func TestEmojiTokensAreForManagers(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	admin := readyAs(t, h, 1, "admin")
	bob := readyAs(t, h, 2, "bob")
	addEmoji(t, h, "taken")
	h.EmojiChanged()
	next(t, admin)
	next(t, bob)
	bob.Handle([]byte(`{"type":"emoji-token","name":"party"}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	for _, bad := range []string{"Party", "a", "taken"} {
		admin.Handle([]byte(`{"type":"emoji-token","name":"` + bad + `"}`))
		if m := next(t, admin); m["type"] != "error" || m["about"] != "manage" {
			t.Fatalf("%q: %v", bad, m)
		}
	}
	admin.Handle([]byte(`{"type":"emoji-token","name":"party"}`))
	m := next(t, admin)
	token, _ := m["token"].(string)
	if m["type"] != "upload-token" || token == "" {
		t.Fatalf("got %v", m)
	}
	// For one emoji upload, not for files.
	if author, name, ok := h.RedeemEmojiToken(token); !ok || author != "admin" || name != "party" {
		t.Fatalf("redeem: %q %q %v", author, name, ok)
	}
	if _, _, ok := h.RedeemEmojiToken(token); ok {
		t.Fatal("redeemed twice")
	}
	admin.Handle([]byte(`{"type":"upload-token"}`))
	if _, _, ok := h.RedeemEmojiToken(next(t, admin)["token"].(string)); ok {
		t.Fatal("a file token added an emoji")
	}
	admin.Handle([]byte(`{"type":"emoji-token","name":"party"}`))
	if _, ok := h.RedeemUploadToken(next(t, admin)["token"].(string)); ok {
		t.Fatal("an emoji token uploaded a file")
	}
}

func TestEmojiTokensAreRefusedAtTheLimit(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	admin := readyAs(t, h, 1, "admin")
	for i := range store.MaxEmoji {
		addEmoji(t, h, fmt.Sprintf("e%d", i))
	}
	h.EmojiChanged()
	next(t, admin)
	admin.Handle([]byte(`{"type":"emoji-token","name":"more"}`))
	if m := next(t, admin); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}

func TestEmojiTokensExpire(t *testing.T) {
	now := clock
	h := newHub(t, "general")
	h.now = func() time.Time { return now }
	h.SetVoice(&fakeVoice{}, "")
	admin := readyAs(t, h, 1, "admin")
	admin.Handle([]byte(`{"type":"emoji-token","name":"party"}`))
	token := next(t, admin)["token"].(string)
	now = now.Add(tokenLifetime + time.Second)
	if _, _, ok := h.RedeemEmojiToken(token); ok {
		t.Fatal("expired token redeemed")
	}
}

func TestManagersDeleteEmoji(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	addEmoji(t, h, "party")
	h.EmojiChanged()
	admin := readyAs(t, h, 1, "admin")
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"delete-emoji","name":"party"}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"delete-emoji","name":"party"}`))
	for _, c := range []*Conn{admin, bob} {
		if m := next(t, c); m["type"] != "emoji" || len(m["emoji"].([]any)) != 0 {
			t.Fatalf("got %v", m)
		}
	}
	admin.Handle([]byte(`{"type":"delete-emoji","name":"party"}`))
	if m := next(t, admin); m["type"] != "error" {
		t.Fatalf("deleted twice: %v", m)
	}
}

func TestCustomEmojiReactions(t *testing.T) {
	h := newHub(t, "general")
	addEmoji(t, h, "top10")
	h.EmojiChanged()
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true, UserID: 4})
	id := say(t, h, alice, "hi")
	react(alice, id, ":nope:", true)
	if m := next(t, alice); m["type"] != "error" {
		t.Fatalf("unknown emoji: %v", m)
	}
	react(alice, id, ":top10:", true)
	m := next(t, alice)
	if rs := m["reactions"].([]any); m["type"] != "reactions" || rs[0].(map[string]any)["emoji"] != ":top10:" {
		t.Fatalf("got %v", m)
	}
}
