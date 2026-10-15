package sidecar

import (
	"strconv"
	"strings"
	"testing"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func react(c *Conn, id int64, emoji string, on bool) {
	c.Handle([]byte(`{"type":"react","message":` + strconv.FormatInt(id, 10) + `,"emoji":"` + emoji + `","on":` + strconv.FormatBool(on) + `}`))
}

func TestReactionsGoToEveryone(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true})
	guest := joinAs(t, h, mumble.Identity{Session: 2, Name: "guest"})
	id := say(t, h, alice, "hi")
	next(t, guest)
	react(guest, id, "👍", true)
	for _, c := range []*Conn{alice, guest} {
		m := next(t, c)
		rs := m["reactions"].([]any)
		if m["type"] != "reactions" || m["message"] != float64(id) || m["channel"] != float64(h.channels[0].ID) || len(rs) != 1 {
			t.Fatalf("got %v", m)
		}
		if r := rs[0].(map[string]any); r["emoji"] != "👍" || r["users"].([]any)[0] != "guest" {
			t.Fatalf("reaction %v", r)
		}
	}
	react(guest, id, "👍", false)
	if m := next(t, alice); len(m["reactions"].([]any)) != 0 {
		t.Fatalf("after removing: %v", m)
	}
	next(t, guest)
	// History has them.
	react(alice, id, "🎉", true)
	next(t, alice)
	next(t, guest)
	alice.Handle([]byte(`{"type":"history","channel":` + strconv.FormatInt(h.channels[0].ID, 10) + `}`))
	msgs := next(t, alice)["messages"].([]any)
	if rs := msgs[len(msgs)-1].(map[string]any)["reactions"].([]any); len(rs) != 1 {
		t.Fatalf("history %v", msgs)
	}
}

func TestOnlyEmojiAreReactions(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true})
	alice.limit = rate.NewLimiter(rate.Inf, 0) // many requests in a row
	id := say(t, h, alice, "hi")
	for _, bad := range []string{"", "lol", "<b>", " 👍", "\u200b", "★ПРИВЕТ", "ＦＵＣＫ★", "👍\u0489\u0489", "👍\u200d", "\u200d★", "👍\u200d\u200d👍", strings.Repeat("👍", 17)} {
		react(alice, id, bad, true)
		if m := next(t, alice); m["type"] != "error" {
			t.Fatalf("%q: %v", bad, m)
		}
	}
	for _, good := range []string{"❤️", "👍🏽", "👨‍👩‍👧", "🇩🇪", "🧑🏻‍❤️‍💋‍🧑🏼", "🏴\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F"} {
		react(alice, id, good, true)
		if m := next(t, alice); m["type"] != "reactions" {
			t.Fatalf("%q: %v", good, m)
		}
	}
	react(alice, 9999, "👍", true)
	if m := next(t, alice); m["type"] != "error" {
		t.Fatalf("unknown message: %v", m)
	}
}

func TestReactingInAVoiceChatNeedsWrite(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	chat, _ := h.store.VoiceChat(9) // session 2 may not use 9
	m, _ := h.store.AddMessage(store.NewMessage{ChannelID: chat, Author: "mod", Text: "x", At: clock})
	other := readyAs(t, h, 2, "other")
	react(other, m.ID, "👍", true)
	if got := next(t, other); got["type"] != "error" {
		t.Fatalf("reacted without write: %v", got)
	}
}

func TestSameEmojiInOtherFormsIsOne(t *testing.T) {
	for in, want := range map[string]string{"❤": "❤\ufe0f", "❤\ufe0f": "❤\ufe0f", "👍\ufe0f": "👍", "✅": "✅\ufe0f"} {
		if got := normalEmoji(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

func TestNoEventWithoutAChange(t *testing.T) {
	h := newHub(t, "general")
	alice := joinAs(t, h, mumble.Identity{Session: 1, Name: "alice", Registered: true})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true})
	id := say(t, h, alice, "hi")
	next(t, bob)
	react(alice, id, "👍", true)
	next(t, alice)
	next(t, bob)
	react(alice, id, "👍", true)  // already there
	react(alice, id, "🎉", false) // never there
	empty(t, alice)
	empty(t, bob)
}
