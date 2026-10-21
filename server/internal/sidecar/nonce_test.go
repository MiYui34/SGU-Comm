package sidecar

import (
	"strings"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

func TestSendWithANonce(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true, UserID: 3})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 4})
	send := `{"type":"send","channel":1,"text":"hi","nonce":"n1"}`
	alice.Handle([]byte(send))
	mine, theirs := next(t, alice), next(t, bob)
	id := mine["message"].(map[string]any)["id"]
	// Only the sender sees the nonce.
	if mine["type"] != "message" || mine["nonce"] != "n1" || theirs["type"] != "message" || theirs["nonce"] != nil {
		t.Fatalf("alice %v, bob %v", mine, theirs)
	}
	// A retry (from a new connection after a reconnect) stores nothing new:
	// the sender gets the message again, nobody else hears of it.
	alice.Close()
	again := joinAs(t, h, mumble.Identity{Session: 5, Name: "alice", Registered: true, UserID: 3})
	again.Handle([]byte(send))
	if m := next(t, again); m["type"] != "message" || m["nonce"] != "n1" || m["message"].(map[string]any)["id"] != id {
		t.Fatalf("retry %v", m)
	}
	empty(t, bob)
	if hist, _ := h.store.History(1, 0, 10); len(hist) != 1 {
		t.Fatalf("stored %d messages", len(hist))
	}
	// Also when slow mode would refuse a new message now.
	h.store.SetSlowMode(1, 60)
	again.Handle([]byte(send))
	if m := next(t, again); m["type"] != "message" {
		t.Fatalf("retry in slow mode %v", m)
	}
	// Bob's own nonce of the same value is his.
	bob.Handle([]byte(send))
	if m := next(t, bob); m["type"] != "message" || m["message"].(map[string]any)["id"] == id {
		t.Fatalf("bob %v", m)
	}
}

func TestSendWithoutANonceAsBefore(t *testing.T) {
	h := newHub(t, "general")
	alice, bob := ready(t, h, "alice"), ready(t, h, "bob")
	for range 2 {
		alice.Handle([]byte(`{"type":"send","channel":1,"text":"hi"}`))
		for _, c := range []*Conn{alice, bob} {
			if m := next(t, c); m["type"] != "message" || m["nonce"] != nil {
				t.Fatalf("got %v", m)
			}
		}
	}
	if hist, _ := h.store.History(1, 0, 10); len(hist) != 2 {
		t.Fatalf("stored %d messages", len(hist))
	}
}

func TestLongNoncesAreRefused(t *testing.T) {
	h := newHub(t, "general")
	alice := ready(t, h, "alice")
	alice.Handle([]byte(`{"type":"send","channel":1,"text":"hi","nonce":"` + strings.Repeat("x", maxNonce+1) + `"}`))
	if m := next(t, alice); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}
