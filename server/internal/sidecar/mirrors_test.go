package sidecar

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

func TestMirrorsFollowTheTextChannels(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	admin := h.Connect()
	t.Cleanup(admin.Close)
	admin.SetIdentity(mumble.Identity{Session: 1, Name: "admin"})
	ready := next(t, admin)
	chans := ready["channels"].([]any)
	if chans[0].(map[string]any)["mumble"] != float64(100) {
		t.Fatalf("ready %v", ready)
	}
	admin.Handle([]byte(`{"type":"create-channel","name":"games"}`))
	m := next(t, admin)
	chans = m["channels"].([]any)
	if len(v.synced) != 2 || v.synced[0] != (Mirror{Channel: 100, Name: "general"}) || v.synced[1].Name != "games" ||
		chans[1].(map[string]any)["mumble"] != float64(101) {
		t.Fatalf("synced %v, channels %v", v.synced, m)
	}
}

func TestMirrorsCarryMessagesBothWays(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	web := readyAs(t, h, 2, "web")
	general := int64(h.channels[0].ID)
	// From a Mumble client in #general to the web.
	h.MumbleText(9, "bob", true, 100, "hi from mumble")
	if m := next(t, web); m["type"] != "message" || m["message"].(map[string]any)["channel"] != float64(general) {
		t.Fatalf("web got %v", m)
	}
	// From the web to the Mumble clients in #general.
	web.Handle([]byte(`{"type":"send","channel":` + strconv.FormatInt(general, 10) + `,"text":"hi from web"}`))
	next(t, web)
	if got := v.toChannel[100]; len(got) != 1 || got[0] != "hi from web" {
		t.Fatalf("to Mumble: %v", v.toChannel)
	}
}

func TestMumbleUsersGetWhatTheyMissed(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	for i := range 25 {
		h.MumbleText(9, "bob", false, 100, fmt.Sprintf("m%d", i))
	}
	// First visit in this connection: the last 20, under a heading.
	h.UserMoved(7, noChannel, 0)
	h.UserMoved(7, 0, 100)
	got := v.userTexts(7)
	if len(got) != 1 || strings.Count(got[0], "<br>") != 20 || !strings.Contains(got[0], "m24") || strings.Contains(got[0], "m4<") {
		t.Fatalf("first visit: %q", got)
	}
	// Back later: only what came while away.
	h.UserMoved(7, 100, 0)
	h.MumbleText(9, "bob", false, 100, "while away")
	h.UserMoved(7, 0, 100)
	if got := v.userTexts(7); len(got) != 2 || strings.Count(got[1], "<br>") != 1 || !strings.Contains(got[1], "while away") {
		t.Fatalf("second visit: %q", got)
	}
	// Nothing new: nothing sent.
	h.UserMoved(7, 100, 0)
	h.UserMoved(7, 0, 100)
	if got := v.userTexts(7); len(got) != 2 {
		t.Fatalf("third visit: %q", got)
	}
	// A new connection with that session starts over.
	h.UserMoved(7, 100, noChannel)
	h.UserMoved(7, noChannel, 100)
	if got := v.userTexts(7); len(got) != 3 || strings.Count(got[2], "<br>") != 20 {
		t.Fatalf("new connection: %q", got)
	}
}

func TestNoMirrorsWithoutTheBuiltInServer(t *testing.T) {
	// The server ran with the built-in Mumble server before, now with
	// Murmur: Murmur's channels with those ids are not mirrors.
	h := newHub(t, "general")
	h.store.SetMumbleChannel(h.channels[0].ID, 5)
	h2, err := NewHub(h.store, h.now)
	if err != nil {
		t.Fatal(err)
	}
	c := h2.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 1, Name: "x"})
	ready := next(t, c)
	if _, ok := ready["channels"].([]any)[0].(map[string]any)["mumble"]; ok {
		t.Fatalf("ready %v", ready)
	}
}

func TestTopics(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	admin := joinAs(t, h, mumble.Identity{Session: 1, Name: "admin", Registered: true})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	general := strconv.FormatInt(h.channels[0].ID, 10)
	bob.Handle([]byte(`{"type":"set-topic","channel":` + general + `,"text":"mine"}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob: %v", m)
	}
	admin.Handle([]byte(`{"type":"set-topic","channel":` + general + `,"text":"  Anything <goes>  "}`))
	m := next(t, admin)
	if ch := m["channels"].([]any)[0].(map[string]any); m["type"] != "channels" || ch["topic"] != "Anything <goes>" {
		t.Fatalf("channels %v", m)
	}
	if v.synced[0].Topic != "Anything <goes>" {
		t.Fatalf("mirror %+v", v.synced)
	}
	long := strings.Repeat("x", 201)
	admin.Handle([]byte(`{"type":"set-topic","channel":` + general + `,"text":"` + long + `"}`))
	if m := next(t, admin); m["type"] != "error" {
		t.Fatalf("long topic: %v", m)
	}
}
