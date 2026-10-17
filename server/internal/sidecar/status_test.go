package sidecar

import (
	"strconv"
	"strings"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func TestEffectiveStatus(t *testing.T) {
	now := clock.UnixMilli()
	for _, c := range []struct {
		st   store.UserStatus
		away bool
		want statusView
	}{
		{store.UserStatus{}, false, statusView{Status: "online"}},
		{store.UserStatus{}, true, statusView{Status: "idle"}},
		{store.UserStatus{Status: "idle"}, false, statusView{Status: "idle"}},
		{store.UserStatus{Status: "dnd"}, true, statusView{Status: "dnd"}},
		{store.UserStatus{Emoji: "🍕", Text: "lunch", Until: now + 1}, false, statusView{Status: "online", Emoji: "🍕", Text: "lunch", Until: now + 1}},
		// After its end, the custom status is gone.
		{store.UserStatus{Status: "dnd", Emoji: "🍕", Text: "lunch", Until: now}, false, statusView{Status: "dnd"}},
	} {
		if got := effective(c.st, c.away, clock); got != c.want {
			t.Errorf("effective(%+v, %v) = %+v, want %+v", c.st, c.away, got, c.want)
		}
	}
}

func TestStatusGoesToEveryoneAndIsKept(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true, UserID: 4})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob"})
	until := strconv.FormatInt(clock.UnixMilli()+60_000, 10)
	alice.Handle([]byte(`{"type":"set-status","status":"dnd","emoji":"🍕","text":"  at   lunch ","until":` + until + `}`))
	if m := next(t, alice); m["type"] != "own-status" || m["status"] != "dnd" || m["text"] != "at lunch" {
		t.Fatalf("own: %v", m)
	}
	for _, c := range []*Conn{alice, bob} {
		if m := next(t, c); m["type"] != "status" || m["session"] != 1.0 || m["status"] != "dnd" || m["emoji"] != "🍕" || m["until"] == nil {
			t.Fatalf("event: %v", m)
		}
	}
	// A second device of alice gets it with ready; the others see it.
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 3, Name: "alice", Registered: true, UserID: 4})
	m := next(t, c)
	own := m["ownStatus"].(map[string]any)
	statuses := m["statuses"].([]any)
	if own["status"] != "dnd" || own["text"] != "at lunch" || len(statuses) != 2 {
		t.Fatalf("ready: %v", m)
	}
	if m := next(t, bob); m["type"] != "status" || m["session"] != 3.0 {
		t.Fatalf("second device: %v", m)
	}
	// A change on one device changes all of them.
	next(t, alice)
	c.Handle([]byte(`{"type":"set-status","status":"online"}`))
	for _, conn := range []*Conn{alice, c} {
		if m := next(t, conn); m["type"] != "own-status" || m["status"] != "online" {
			t.Fatalf("own on all devices: %v", m)
		}
	}
	if st, _ := h.store.UserStatus(4); st != (store.UserStatus{}) {
		t.Fatalf("stored %+v", st)
	}
}

func TestAwayIsIdleUnlessDoNotDisturb(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice"})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob"})
	alice.Handle([]byte(`{"type":"away","on":true}`))
	if m := next(t, bob); m["type"] != "status" || m["status"] != "idle" {
		t.Fatalf("away: %v", m)
	}
	next(t, alice)
	alice.Handle([]byte(`{"type":"set-status","status":"dnd"}`))
	next(t, alice) // own-status
	if m := next(t, bob); m["status"] != "dnd" {
		t.Fatalf("dnd: %v", m)
	}
	next(t, alice)
	// Back from away: still dnd, nothing to tell.
	alice.Handle([]byte(`{"type":"away","on":false}`))
	empty(t, bob)
	// A guest's status leaves with it.
	alice.Close()
	if m := next(t, bob); m["type"] != "status" || m["session"] != 1.0 || m["status"] != "online" {
		t.Fatalf("gone: %v", m)
	}
}

func TestBadStatusesAreRefused(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice"})
	for _, req := range []string{
		`{"type":"set-status","status":"invisible"}`,
		`{"type":"set-status","status":"online","emoji":"x"}`,
		`{"type":"set-status","status":"online","text":"` + strings.Repeat("a", maxStatusText+1) + `"}`,
	} {
		alice.Handle([]byte(req))
		if m := next(t, alice); m["type"] != "error" {
			t.Fatalf("%s: %v", req, m)
		}
	}
}
