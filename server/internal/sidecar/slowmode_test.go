package sidecar

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// slowChannel is a channel's slow mode in a channel list.
func slowChannel(t *testing.T, list any, i int) any {
	t.Helper()
	return list.([]any)[i].(map[string]any)["slowMode"]
}

func TestSlowModeIsSetByChannelEditors(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	admin, bob := readyAs(t, h, 1, "admin"), readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"set-slow-mode","channel":1,"seconds":30}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"set-slow-mode","channel":1,"seconds":7}`))
	if m := next(t, admin); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("an odd value: %v", m)
	}
	admin.Handle([]byte(`{"type":"set-slow-mode","channel":1,"seconds":30}`))
	for _, c := range []*Conn{admin, bob} {
		if m := next(t, c); m["type"] != "channels" || slowChannel(t, m["channels"], 0) != float64(30) {
			t.Fatalf("got %v", m)
		}
	}
	if chans, _ := h.store.Channels(); chans[0].SlowMode != 30 {
		t.Fatalf("saved %d", chans[0].SlowMode)
	}
	late := h.Connect()
	t.Cleanup(late.Close)
	late.SetIdentity(mumble.Identity{Session: 3, Name: "late"})
	if m := next(t, late); slowChannel(t, m["channels"], 0) != float64(30) {
		t.Fatalf("ready %v", m)
	}
}

func TestSlowModeWaitsPerUserAndChannel(t *testing.T) {
	h := newHub(t, "general", "random")
	h.SetVoice(&fakeVoice{}, "")
	now := clock
	h.now = func() time.Time { return now }
	admin, bob, carol := readyAs(t, h, 1, "admin"), readyAs(t, h, 2, "bob"), readyAs(t, h, 3, "carol")
	// A guest with the same name is someone else; dave posts before slow
	// mode starts.
	bob2, dave := readyAs(t, h, 4, "bob"), readyAs(t, h, 5, "dave")
	send := func(c *Conn, channel int, text string) map[string]any {
		t.Helper()
		c.Handle(fmt.Appendf(nil, `{"type":"send","channel":%d,"text":%q}`, channel, text))
		m := next(t, c)
		drain(admin, bob, carol, bob2, dave)
		return m
	}
	send(dave, 1, "early")
	if err := h.store.SetSlowMode(1, 30); err != nil {
		t.Fatal(err)
	}
	if err := h.reloadChannels(); err != nil {
		t.Fatal(err)
	}
	if m := send(dave, 1, "now"); m["type"] != "error" {
		t.Fatalf("dave: %v", m)
	}
	first := send(bob, 1, "first")
	if first["type"] != "message" {
		t.Fatalf("first: %v", first)
	}
	now = now.Add(7500 * time.Millisecond)
	m := send(bob, 1, "again")
	if m["type"] != "error" || m["message"] != "Slow mode: wait 23 s" || m["channel"] != float64(1) ||
		m["until"] != float64(clock.Add(30*time.Second).UnixMilli()) || m["about"] != "slow-mode" {
		t.Fatalf("again: %v", m)
	}
	// Others, other channels and staff are not held up.
	for _, s := range []struct {
		c       *Conn
		channel int
	}{{carol, 1}, {bob2, 1}, {bob, 2}, {admin, 1}, {admin, 1}} {
		if m := send(s.c, s.channel, "hi"); m["type"] != "message" {
			t.Fatalf("%v", m)
		}
	}
	// Edits do not count.
	bob.Handle(fmt.Appendf(nil, `{"type":"edit-message","message":%v,"text":"edited"}`, first["message"].(map[string]any)["id"]))
	if m := next(t, bob); m["type"] != "message-edited" {
		t.Fatalf("edit: %v", m)
	}
	drain(admin, bob, carol)
	now = clock.Add(30 * time.Second)
	if m := send(bob, 1, "later"); m["type"] != "message" {
		t.Fatalf("later: %v", m)
	}
}

func TestSlowModeCountsUploads(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	bob := readyAs(t, h, 2, "bob")
	h.store.SetSlowMode(1, 60)
	h.reloadChannels()
	for _, key := range []string{"a", "b"} {
		if _, err := h.store.AddUpload(store.Upload{Key: key, Name: "cat.png", Type: "image/png", Size: 3, Author: "bob"}, clock); err != nil {
			t.Fatal(err)
		}
	}
	bob.Handle([]byte(`{"type":"send","channel":1,"text":"","upload":"a"}`))
	if m := next(t, bob); m["type"] != "message" {
		t.Fatalf("upload: %v", m)
	}
	for _, req := range []string{`{"type":"send","channel":1,"text":"hi"}`, `{"type":"send","channel":1,"text":"","upload":"b"}`} {
		bob.Handle([]byte(req))
		if m := next(t, bob); m["type"] != "error" || !strings.HasPrefix(m["message"].(string), "Slow mode") {
			t.Fatalf("%s: %v", req, m)
		}
	}
}

func TestSlowModeHoldsMumbleClientsInMirrors(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{users: map[uint32]MumbleUser{9: {Registered: true, UserID: 504}}}
	h.SetVoice(v, "")
	h.store.SetSlowMode(1, 5)
	h.reloadChannels()
	// Mirror 100 is #general's; channel 5 is a voice channel.
	if err := h.MumbleTextAllowed(9, 100); err != nil {
		t.Fatal(err)
	}
	h.MumbleText(mumble.Identity{Session: 9, Name: "bob", Registered: true, UserID: 504}, 100, "hi", nil)
	if err := h.MumbleTextAllowed(9, 100); err == nil || err.Error() != "Slow mode: wait 5 s" {
		t.Fatalf("again: %v", err)
	}
	// The same user on the web waits too.
	web := h.Connect()
	t.Cleanup(web.Close)
	web.SetIdentity(mumble.Identity{Session: 10, Name: "bob", Registered: true, UserID: 504})
	next(t, web)
	web.Handle([]byte(`{"type":"send","channel":1,"text":"hi"}`))
	if m := next(t, web); m["type"] != "error" {
		t.Fatalf("web: %v", m)
	}
	if h.MumbleTextAllowed(9, 5) != nil || h.MumbleTextAllowed(1, 100) != nil || h.MumbleTextAllowed(1, 100) != nil {
		t.Fatal("voice channels and staff are free")
	}
}
