package sidecar

import (
	"strconv"
	"strings"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

// directHub has three registered users: alice (10), bob (20), carol (30).
func directHub(t *testing.T) (*Hub, *Conn, *Conn, *Conn) {
	t.Helper()
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{names: map[uint32]string{10: "alice", 20: "bob", 30: "carol"}}, "")
	alice := joinAs(t, h, mumble.Identity{Session: 3, Name: "alice", Registered: true, UserID: 10})
	bob := joinAs(t, h, mumble.Identity{Session: 4, Name: "bob", Registered: true, UserID: 20})
	carol := joinAs(t, h, mumble.Identity{Session: 5, Name: "carol", Registered: true, UserID: 30})
	return h, alice, bob, carol
}

// openDirect asks for the chat with a user and returns its id.
func openDirect(t *testing.T, c *Conn, user int) string {
	t.Helper()
	c.Handle([]byte(`{"type":"dm","user":` + strconv.Itoa(user) + `}`))
	m := next(t, c)
	if m["type"] != "dm" {
		t.Fatalf("dm: %v", m)
	}
	return strconv.FormatFloat(m["channel"].(float64), 'f', -1, 64)
}

func peerOf(m map[string]any) (float64, any) {
	p := m["peer"].(map[string]any)
	return p["id"].(float64), p["name"]
}

func TestDirectMessagesReachOnlyTheTwo(t *testing.T) {
	h, alice, bob, carol := directHub(t)
	chat := openDirect(t, alice, 20)
	alice.Handle([]byte(`{"type":"send","channel":` + chat + `,"text":"hi bob"}`))
	for c, peer := range map[*Conn]float64{alice: 20, bob: 10} {
		info := next(t, c)
		if id, _ := peerOf(info); info["type"] != "dm" || id != peer {
			t.Fatalf("dm info %v", info)
		}
		if m := next(t, c); m["type"] != "message" || m["message"].(map[string]any)["text"] != "hi bob" {
			t.Fatalf("message %v", m)
		}
	}
	empty(t, carol)
	for _, req := range []string{
		`{"type":"history","channel":` + chat + `}`,
		`{"type":"send","channel":` + chat + `,"text":"me too"}`,
	} {
		carol.Handle([]byte(req))
		if m := next(t, carol); m["type"] != "error" {
			t.Fatalf("carol: %v", m)
		}
	}
	// Bob comes back later: the chat is listed, with one unread message.
	again := h.Connect()
	t.Cleanup(again.Close)
	again.SetIdentity(mumble.Identity{Session: 6, Name: "bob", Registered: true, UserID: 20})
	ready := next(t, again)
	directs := ready["directs"].([]any)
	if len(directs) != 1 {
		t.Fatalf("directs %v", ready)
	}
	if id, name := peerOf(directs[0].(map[string]any)); id != 10 || name != "alice" {
		t.Fatalf("directs %v", directs)
	}
	var found bool
	for _, e := range ready["summary"].([]any) {
		e := e.(map[string]any)
		if strconv.FormatFloat(e["channel"].(float64), 'f', -1, 64) == chat {
			found = e["mentions"] == float64(1) && e["direct"] == float64(10)
		}
	}
	if !found {
		t.Fatalf("summary %v", ready["summary"])
	}
	// Search: bob finds it, carol does not.
	bob.Handle([]byte(`{"type":"search","text":"hi"}`))
	if m := next(t, bob); len(m["messages"].([]any)) != 1 {
		t.Fatalf("bob search %v", m)
	}
	carol.Handle([]byte(`{"type":"search","text":"hi"}`))
	if m := next(t, carol); len(m["messages"].([]any)) != 0 {
		t.Fatalf("carol search %v", m)
	}
}

func TestDirectChatRefusals(t *testing.T) {
	h, alice, _, _ := directHub(t)
	guest := joinAs(t, h, mumble.Identity{Session: 7, Name: "guest"})
	for _, tc := range []struct {
		c    *Conn
		user int
	}{{alice, 10}, {alice, 99}, {guest, 10}} {
		tc.c.Handle([]byte(`{"type":"dm","user":` + strconv.Itoa(tc.user) + `}`))
		if m := next(t, tc.c); m["type"] != "error" {
			t.Fatalf("dm to %d: %v", tc.user, m)
		}
	}
}

// In a direct chat both users pin (not only moderators); others cannot.
func TestBothUsersPinInADirectChat(t *testing.T) {
	_, alice, bob, carol := directHub(t)
	chat := openDirect(t, alice, 20)
	alice.Handle([]byte(`{"type":"send","channel":` + chat + `,"text":"keep this"}`))
	next(t, alice)
	m := next(t, alice)
	id := strconv.FormatFloat(m["message"].(map[string]any)["id"].(float64), 'f', -1, 64)
	next(t, bob)
	next(t, bob)
	bob.Handle([]byte(`{"type":"pin","message":` + id + `,"on":true}`))
	if m := next(t, bob); m["type"] != "message-pinned" {
		t.Fatalf("bob pin: %v", m)
	}
	next(t, alice)
	carol.Handle([]byte(`{"type":"pin","message":` + id + `,"on":false}`))
	if m := next(t, carol); m["type"] != "error" {
		t.Fatalf("carol unpinned: %v", m)
	}
}

func TestMumblePrivateTextsAreStored(t *testing.T) {
	h, alice, bob, carol := directHub(t)
	h.MumblePrivateText(mumble.Identity{Session: 50, Name: "alice", Registered: true, UserID: 10}, 20, "from my Mumble client", nil, false)
	for _, c := range []*Conn{alice, bob} {
		if m := next(t, c); m["type"] != "dm" {
			t.Fatalf("dm info %v", m)
		}
		if m := next(t, c); m["message"].(map[string]any)["text"] != "from my Mumble client" {
			t.Fatalf("message %v", m)
		}
	}
	empty(t, carol)
}

func TestDirectMessagesReachMumbleClients(t *testing.T) {
	h, alice, bob, _ := directHub(t)
	v := h.voice.(*fakeVoice)
	v.native = map[uint32]bool{20: true} // bob is also on a Mumble client
	chat := openDirect(t, alice, 20)
	alice.Handle([]byte(`{"type":"send","channel":` + chat + `,"text":"hi bob"}`))
	next(t, alice)
	m := next(t, alice)
	next(t, bob)
	next(t, bob)
	if got := v.privateTexts(); len(got) != 1 || got[0] != "3>20:hi bob" {
		t.Fatalf("private texts %v", got)
	}
	// Seen there: read (no catch-up later).
	ch, _ := strconv.ParseInt(chat, 10, 64)
	if mark, _, _ := h.store.ReadMark(20, ch); mark != int64(m["message"].(map[string]any)["id"].(float64)) {
		t.Fatalf("read mark %d", mark)
	}
}

func TestMumbleClientsGetMissedDirectMessages(t *testing.T) {
	h, alice, _, _ := directHub(t)
	v := h.voice.(*fakeVoice)
	chat := openDirect(t, alice, 20)
	alice.Handle([]byte(`{"type":"send","channel":` + chat + `,"text":"while you were out"}`))
	v.users = map[uint32]MumbleUser{60: {Registered: true, UserID: 20}}
	h.UserMoved(60, noChannel, 0)
	got := v.userTexts(60)
	if len(got) != 1 || !strings.Contains(got[0], "alice") || !strings.Contains(got[0], "while you were out") {
		t.Fatalf("got %q", got)
	}
	// Once only.
	h.UserMoved(60, 0, noChannel)
	h.UserMoved(60, noChannel, 0)
	if got := v.userTexts(60); len(got) != 1 {
		t.Fatalf("again %q", got)
	}
}

// Seen live on a Mumble client: read, no catch-up later.
func TestDeliveredMumblePrivateTextsAreRead(t *testing.T) {
	h, _, _, _ := directHub(t)
	v := h.voice.(*fakeVoice)
	h.MumblePrivateText(mumble.Identity{Session: 50, Name: "alice", Registered: true, UserID: 10}, 20, "seen live", nil, true)
	v.users = map[uint32]MumbleUser{60: {Registered: true, UserID: 20}}
	h.UserMoved(60, noChannel, 0)
	if got := v.userTexts(60); len(got) != 0 {
		t.Fatalf("catch-up repeats %q", got)
	}
}

// A relayed message does not mark older unread ones read: the catch-up
// still has them.
func TestRelayKeepsOlderUnread(t *testing.T) {
	h, alice, _, _ := directHub(t)
	v := h.voice.(*fakeVoice)
	chat := openDirect(t, alice, 20)
	alice.Handle([]byte(`{"type":"send","channel":` + chat + `,"text":"first, while away"}`))
	v.native = map[uint32]bool{20: true} // bob's Mumble client is there now
	alice.Handle([]byte(`{"type":"send","channel":` + chat + `,"text":"second, live"}`))
	v.users = map[uint32]MumbleUser{60: {Registered: true, UserID: 20}}
	h.UserMoved(60, noChannel, 0)
	if got := v.userTexts(60); len(got) != 1 || !strings.Contains(got[0], "first, while away") {
		t.Fatalf("catch-up %q", got)
	}
}
