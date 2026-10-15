package sidecar

import (
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func TestMentionRule(t *testing.T) {
	for text, want := range map[string]bool{
		"@lain hi": true, "hi @Lain!": true, "(@lain)": true,
		"@lainey": false, "mail@lain.example": false, "lain": false,
	} {
		if got := mentionsName(text, "lain"); got != want {
			t.Errorf("%q: %v", text, got)
		}
	}
}

func TestReadyCarriesReadMarksAndASummary(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	general := h.channels[0].ID
	for _, text := range []string{"old", "@alice old", "new", "@alice new", "@bob not me"} {
		h.store.AddMessage(store.NewMessage{ChannelID: general, Author: "bob", Text: text, At: clock})
	}
	own, _ := h.store.AddMessage(store.NewMessage{ChannelID: general, Author: "alice", Verified: true, Text: "@alice myself", At: clock})
	h.store.SetReadMark("alice", general, 2)
	chat, _ := h.store.VoiceChat(5)
	h.store.AddMessage(store.NewMessage{ChannelID: chat, Author: "bob", Text: "@alice in voice", At: clock})

	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 1, Name: "Alice", Registered: true})
	ready := next(t, c)
	reads := ready["reads"].(map[string]any)
	if reads[strconv.FormatInt(general, 10)] != float64(2) {
		t.Fatalf("reads %v", ready["reads"])
	}
	byChannel := map[float64]map[string]any{}
	for _, e := range ready["summary"].([]any) {
		byChannel[e.(map[string]any)["channel"].(float64)] = e.(map[string]any)
	}
	g := byChannel[float64(general)]
	if g["latest"] != float64(own.ID) || g["mentions"] != float64(1) {
		t.Fatalf("general %v", g)
	}
	if v := byChannel[float64(chat)]; v["voice"] != float64(5) || v["mentions"] != float64(1) {
		t.Fatalf("voice chat %v", v)
	}
}

func TestMarkRead(t *testing.T) {
	h := newHub(t, "general")
	general := h.channels[0].ID
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "Alice", Registered: true})
	alice.Handle([]byte(`{"type":"mark-read","channel":` + strconv.FormatInt(general, 10) + `,"message":7}`))
	guest := joinAs(t, h, mumble.Identity{Session: 2, Name: "guest"})
	guest.Handle([]byte(`{"type":"mark-read","channel":` + strconv.FormatInt(general, 10) + `,"message":7}`))
	if m, _ := h.store.ReadMarks("alice"); m[general] != 7 {
		t.Fatalf("alice marks %v", m)
	}
	if m, _ := h.store.ReadMarks("guest"); len(m) != 0 {
		t.Fatalf("guest marks %v", m)
	}
}

func TestMentionRuleKnowsUnicodeSpaces(t *testing.T) {
	if !mentionsName("hi @lain", "lain") || !mentionsName("　@lain", "lain") {
		t.Fatal("no mention after a no-break or wide space")
	}
}

func TestSearch(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	general := h.channels[0].ID
	h.store.AddMessage(store.NewMessage{ChannelID: general, Author: "alice", Text: "pizza tonight", At: clock})
	secret, _ := h.store.VoiceChat(9) // session 2 may not read channel 9 in the fake
	h.store.AddMessage(store.NewMessage{ChannelID: secret, Author: "alice", Text: "pizza secret", At: clock})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	bob.Handle([]byte(`{"type":"search","text":"pizza"}`))
	m := next(t, bob)
	msgs := m["messages"].([]any)
	if m["type"] != "search-results" || m["text"] != "pizza" || len(msgs) != 1 || msgs[0].(map[string]any)["text"] != "pizza tonight" {
		t.Fatalf("results %v", m)
	}
	bob.Handle([]byte(`{"type":"search","text":"pizza","author":"carol"}`))
	if m := next(t, bob); len(m["messages"].([]any)) != 0 {
		t.Fatalf("by carol: %v", m)
	}
}
