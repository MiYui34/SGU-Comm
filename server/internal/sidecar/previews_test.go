package sidecar

import (
	"context"
	"strconv"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/preview"
	"kallisti.chat/server/internal/store"
)

type fakePreviews map[string]*preview.Preview

func (f fakePreviews) Get(_ context.Context, url string) *preview.Preview { return f[url] }

func TestPreviewsOfMessages(t *testing.T) {
	h := newHub(t, "general")
	h.Previews = fakePreviews{"https://example.org/a": {URL: "https://example.org/a", Title: "A page", Image: "k1"}}
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true, UserID: 435})
	withLink := say(t, h, alice, "look (https://example.org/a), nice")
	noPreview := say(t, h, alice, "https://example.org/none")
	plain := say(t, h, alice, "no link")
	ids := []int64{withLink, noPreview, plain, 9999}
	req := `{"type":"previews","messages":[`
	for i, id := range ids {
		if i > 0 {
			req += ","
		}
		req += strconv.FormatInt(id, 10)
	}
	alice.Handle([]byte(req + `]}`))
	got := map[float64]any{}
	for range 2 { // messages with a link only
		m := next(t, alice)
		if m["type"] != "preview" {
			t.Fatalf("got %v", m)
		}
		got[m["message"].(float64)] = m["preview"]
	}
	p, ok := got[float64(withLink)].(map[string]any)
	if !ok || p["title"] != "A page" || p["url"] != "https://example.org/a" || p["image"] != "/api/link-image/k1" {
		t.Fatalf("preview %v", got)
	}
	if v, ok := got[float64(noPreview)]; !ok || v != nil {
		t.Fatalf("no preview: %v", got)
	}
	empty(t, alice)
}

func TestNoPreviewsOfChatsWeMayNotRead(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	h.Previews = fakePreviews{"https://example.org/a": {URL: "https://example.org/a", Title: "secret"}}
	chat, _ := h.store.VoiceChat(9) // session 2 may not read 9
	m, _ := h.store.AddMessage(store.NewMessage{ChannelID: chat, Author: "mod", Text: "https://example.org/a", At: clock})
	other := readyAs(t, h, 2, "other")
	other.Handle([]byte(`{"type":"previews","messages":[` + strconv.FormatInt(m.ID, 10) + `]}`))
	empty(t, other)
}

func TestFirstLink(t *testing.T) {
	for text, want := range map[string]string{
		"see https://x.org/a?b=1.":            "https://x.org/a?b=1",
		"<https://x.org/a> and http://y":      "https://x.org/a",
		"**https://x.org**":                   "https://x.org",
		"no link here":                        "",
		"javascript:alert(1)":                 "",
		"||https://x.org|| and https://y.org": "https://y.org",
		"||https://x.org||":                   "",
		"https://x.org||x":                    "https://x.org",
	} {
		if got := firstLink(text); got != want {
			t.Errorf("%q: %q", text, got)
		}
	}
}
