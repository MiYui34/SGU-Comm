package sidecar

import (
	"errors"
	"strings"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

// Session 1 is an admin; sessions in guests are guests.
func (f *fakeVoice) SetGuestsReadOnly(session uint32, on bool) error {
	if session != 1 {
		return errors.New("only admins change this setting")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.guestsReadOnly = on
	return nil
}

func (f *fakeVoice) GuestsReadOnly() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.guestsReadOnly
}

func (f *fakeVoice) ReadOnly(session uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.guestsReadOnly && f.guests[session]
}

func TestGuestsReadOnly(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{guests: map[uint32]bool{3: true}}
	h.SetVoice(v, "")
	admin := readyAs(t, h, 1, "admin")
	bob := readyAs(t, h, 2, "bob")
	guest := h.Connect()
	t.Cleanup(guest.Close)
	guest.SetIdentity(mumble.Identity{Session: 3, Name: "guest", Registered: true, UserID: 30})
	if m := next(t, guest); m["guestsReadOnly"] != false {
		t.Fatalf("ready %v", m)
	}
	guest.Handle([]byte(`{"type":"send","channel":1,"text":"hi"}`))
	if m := next(t, guest); m["type"] != "message" {
		t.Fatalf("off: %v", m)
	}
	next(t, admin)
	next(t, bob)

	bob.Handle([]byte(`{"type":"set-guests-read-only","on":true}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"set-guests-read-only","on":true}`))
	if !v.GuestsReadOnly() {
		t.Fatal("not changed")
	}
	// The voice server reports the change.
	h.GuestsReadOnlyChanged()
	for _, c := range []*Conn{admin, bob, guest} {
		if m := next(t, c); m["type"] != "guests-read-only" || m["on"] != true {
			t.Fatalf("got %v", m)
		}
	}
	for _, req := range []string{
		`{"type":"send","channel":1,"text":"spam"}`,
		`{"type":"react","message":1,"emoji":"x","on":true}`,
		`{"type":"edit-message","message":1,"text":"spam"}`,
		`{"type":"pin","message":1,"on":true}`,
		`{"type":"upload-token"}`,
		`{"type":"dm","user":7}`,
	} {
		guest.Handle([]byte(req))
		if m := next(t, guest); m["type"] != "error" || !strings.Contains(m["message"].(string), "guests") {
			t.Fatalf("%s: %v", req, m)
		}
	}
	// Typing quietly; reading works.
	guest.Handle([]byte(`{"type":"typing","channel":1}`))
	guest.Handle([]byte(`{"type":"history","channel":1}`))
	if m := next(t, guest); m["type"] != "history" {
		t.Fatalf("got %v", m)
	}
	bob.Handle([]byte(`{"type":"send","channel":1,"text":"fine"}`))
	if m := next(t, bob); m["type"] != "message" {
		t.Fatalf("bob got %v", m)
	}
	late := h.Connect()
	t.Cleanup(late.Close)
	late.SetIdentity(mumble.Identity{Session: 4, Name: "late"})
	if m := next(t, late); m["guestsReadOnly"] != true {
		t.Fatalf("ready %v", m)
	}
}
