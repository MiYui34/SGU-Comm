package sidecar

import (
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func TestReplyWithoutAPing(t *testing.T) {
	h := newHub(t, "general")
	general := h.channels[0].ID
	orig, _ := h.store.AddMessage(store.NewMessage{ChannelID: general, Author: "alice", Verified: true, AuthorID: 3, Text: "question", At: clock})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 4})
	reply := `{"type":"send","channel":` + strconv.FormatInt(general, 10) + `,"reply":` + strconv.FormatInt(orig.ID, 10)
	bob.Handle([]byte(reply + `,"text":"quiet answer","noPing":true}`))
	if m := next(t, bob)["message"].(map[string]any); m["noPing"] != true {
		t.Fatalf("quiet reply %v", m)
	}
	bob.Handle([]byte(reply + `,"text":"loud answer"}`))
	if m := next(t, bob)["message"].(map[string]any); m["noPing"] != nil {
		t.Fatalf("reply %v", m)
	}
	// Alice's summary counts only the reply with a ping.
	alice := h.Connect()
	t.Cleanup(alice.Close)
	alice.SetIdentity(mumble.Identity{Session: 1, Name: "alice", Registered: true, UserID: 3})
	if got := next(t, alice)["summary"].([]any)[0].(map[string]any)["mentions"]; got != float64(1) {
		t.Fatalf("mentions %v", got)
	}
}
