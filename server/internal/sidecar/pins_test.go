package sidecar

import (
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

func TestModeratorsPinMessages(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "") // session 1 may manage
	mod := joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 372})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 504})
	id := strconv.FormatInt(say(t, h, bob, "important"), 10)
	next(t, mod)
	ch := strconv.FormatInt(h.channels[0].ID, 10)

	bob.Handle([]byte(`{"type":"pin","message":` + id + `,"on":true}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("bob pinned: %v", m)
	}
	mod.Handle([]byte(`{"type":"pin","message":` + id + `,"on":true}`))
	for _, c := range []*Conn{mod, bob} {
		m := next(t, c)
		if m["type"] != "message-pinned" || m["message"].(map[string]any)["pinned"] == nil {
			t.Fatalf("got %v", m)
		}
	}
	// Pinned again: nothing changes, no event (the next one is the unpin).
	mod.Handle([]byte(`{"type":"pin","message":` + id + `,"on":true}`))
	bob.Handle([]byte(`{"type":"pins","channel":` + ch + `}`))
	m := next(t, bob)
	if msgs := m["messages"].([]any); m["type"] != "pins" || m["channel"] != float64(h.channels[0].ID) || len(msgs) != 1 {
		t.Fatalf("pins %v", m)
	}
	mod.Handle([]byte(`{"type":"pin","message":` + id + `,"on":false}`))
	if m := next(t, bob); m["type"] != "message-pinned" || m["message"].(map[string]any)["pinned"] != nil {
		t.Fatalf("unpin %v", m)
	}
}

func TestPinsOfAVoiceChatNeedRead(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	chat, _ := h.store.VoiceChat(9) // session 2 may not read channel 9 in the fake
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 504})
	bob.Handle([]byte(`{"type":"pins","channel":` + strconv.FormatInt(chat, 10) + `}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}
