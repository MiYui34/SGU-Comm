package sidecar

import (
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

// After a resync (lost events), the client asks for the state again: it
// gets a fresh ready, and nobody else hears of it.
func TestResyncRequestSendsTheStateAgain(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	general := h.channels[0].ID
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "Alice", Registered: true, UserID: 3})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 4})
	drain(alice, bob)
	// What the lost events said: a read mark from another tab.
	if err := h.store.SetReadMark(3, general, 9); err != nil {
		t.Fatal(err)
	}
	alice.Handle([]byte(`{"type":"resync"}`))
	m := next(t, alice)
	if m["type"] != "ready" {
		t.Fatalf("got %v", m)
	}
	if reads := m["reads"].(map[string]any); reads[strconv.FormatInt(general, 10)] != float64(9) {
		t.Fatalf("reads %v", m["reads"])
	}
	for _, key := range []string{"channels", "threads", "follows", "statuses", "ownStatus", "online"} {
		if _, ok := m[key]; !ok {
			t.Errorf("no %s in %v", key, m)
		}
	}
	empty(t, alice)
	empty(t, bob)
}
