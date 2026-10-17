package sidecar

import (
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func TestMentionAllNeedsAModerator(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "") // session 1 may manage
	mod := joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 1})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 2})
	for _, c := range []struct {
		who  *Conn
		text string
		want any
	}{
		{mod, "@everyone meeting", "everyone"},
		{mod, "hi @here", "here"},
		{mod, "@everyone and @here", "everyone"},
		{mod, "mail@everyone.example", nil},
		{bob, "@everyone spam", nil},
		{bob, "@here spam", nil},
	} {
		say(t, h, c.who, c.text)
		other := map[*Conn]*Conn{mod: bob, bob: mod}[c.who]
		if got := next(t, other)["message"].(map[string]any)["mentionAll"]; got != c.want {
			t.Errorf("%q: %v, want %v", c.text, got, c.want)
		}
	}
}

func TestMentionAllInTheSummaryAndAfterAnEdit(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	general := h.channels[0].ID
	add := func(text, all string) store.Message {
		m, _ := h.store.AddMessage(store.NewMessage{ChannelID: general, Author: "mod", Verified: true, AuthorID: 1, Text: text, At: clock, MentionAll: all})
		return m
	}
	add("@everyone one", "everyone")
	add("@here two", "here") // only for who was there
	edited := add("@everyone three", "everyone")
	bob := h.Connect()
	t.Cleanup(bob.Close)
	bob.SetIdentity(mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 2})
	ready := next(t, bob)
	if got := ready["summary"].([]any)[0].(map[string]any)["mentions"]; got != float64(2) {
		t.Fatalf("mentions %v", got)
	}
	// An edit keeps it while the word is there, and cannot add one.
	mod := joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 1})
	for _, c := range []struct {
		text string
		want any
	}{{"@everyone 3", "everyone"}, {"@here 3", nil}, {"three", nil}, {"@everyone again", nil}} {
		mod.Handle([]byte(`{"type":"edit-message","message":` + strconv.FormatInt(edited.ID, 10) + `,"text":"` + c.text + `"}`))
		if m := next(t, mod)["message"].(map[string]any); m["mentionAll"] != c.want {
			t.Fatalf("edit %q: %v", c.text, m)
		}
	}
}

func TestNoMentionAllInDirectChats(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	chat, _ := h.store.DirectChat(1, 2)
	mod := joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 1})
	mod.Handle([]byte(`{"type":"send","channel":` + strconv.FormatInt(chat, 10) + `,"text":"@everyone hi"}`))
	for {
		m := next(t, mod)
		if m["type"] == "message" {
			if all := m["message"].(map[string]any)["mentionAll"]; all != nil {
				t.Fatalf("direct chat: %v", all)
			}
			return
		}
	}
}
