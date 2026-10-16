package sidecar

import (
	"testing"

	"kallisti.chat/server/internal/mumble"
)

func TestStreamTokensAndLiveStreams(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	h.StreamToken = func(session uint32) string { return "token-for-" + string(rune('0'+session)) }
	h.LiveStreams = func() []LiveStream { return []LiveStream{{Session: 4, Channel: 2}} }
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 3, Name: "alice"})
	ready := next(t, c)
	if s := ready["streams"].([]any); len(s) != 1 || s[0].(map[string]any)["session"] != float64(4) {
		t.Fatalf("ready %v", ready)
	}
	c.Handle([]byte(`{"type":"stream-token"}`))
	if m := next(t, c); m["type"] != "stream-token" || m["token"] != "token-for-3" {
		t.Fatalf("token %v", m)
	}
	h.StreamsChanged([]LiveStream{})
	if m := next(t, c); m["type"] != "streams" || len(m["streams"].([]any)) != 0 {
		t.Fatalf("streams %v", m)
	}
}
