package sidecar

import (
	"strconv"
	"testing"
	"time"

	"kallisti.chat/server/internal/mumble"
)

func TestTypingGoesToOthersAtMostEveryFewSeconds(t *testing.T) {
	h := newHub(t, "general")
	now := clock
	h.now = func() time.Time { return now }
	alice, bob := ready(t, h, "alice"), ready(t, h, "bob")
	ch := strconv.FormatInt(h.channels[0].ID, 10)
	typing := `{"type":"typing","channel":` + ch + `}`
	alice.Handle([]byte(typing))
	if m := next(t, bob); m["type"] != "typing" || m["name"] != "alice" || m["channel"] != float64(h.channels[0].ID) {
		t.Fatalf("bob got %v", m)
	}
	empty(t, alice) // not back to the sender
	now = now.Add(time.Second)
	alice.Handle([]byte(typing)) // too soon: dropped, no error
	empty(t, bob)
	empty(t, alice)
	now = now.Add(1900 * time.Millisecond) // the client's 3 s, arriving a bit early
	alice.Handle([]byte(typing))
	if m := next(t, bob); m["type"] != "typing" {
		t.Fatalf("bob got %v", m)
	}
}

func TestTypingInVoiceChatsOnlyForReaders(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	mod, other := readyAs(t, h, 1, "mod"), readyAs(t, h, 2, "other")
	closed := strconv.FormatInt(int64(voiceChat(t, mod, 9)), 10) // session 2 may not read 9
	mod.Handle([]byte(`{"type":"typing","channel":` + closed + `}`))
	empty(t, other)
	// Without write: dropped, without an error (the user only types).
	other.Handle([]byte(`{"type":"typing","channel":` + closed + `}`))
	empty(t, other)
	empty(t, mod)
}

func TestTypingSkipsTheTypistsOtherSessions(t *testing.T) {
	h := newHub(t, "general")
	alice := mumble.Identity{Session: 1, Name: "alice", Registered: true, UserID: 4}
	tab1 := joinAs(t, h, alice)
	alice.Session = 2
	tab2 := joinAs(t, h, alice)
	bob := joinAs(t, h, mumble.Identity{Session: 3, Name: "bob"})
	tab1.Handle([]byte(`{"type":"typing","channel":` + strconv.FormatInt(h.channels[0].ID, 10) + `}`))
	if m := next(t, bob); m["type"] != "typing" {
		t.Fatalf("bob got %v", m)
	}
	empty(t, tab2)
}
