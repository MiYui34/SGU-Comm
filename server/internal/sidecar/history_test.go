package sidecar

import (
	"fmt"
	"strings"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

func say5(h *Hub, channel uint32, texts ...string) {
	for _, t := range texts {
		h.MumbleText(mumble.Identity{Session: 50, Name: "bob", Registered: true, UserID: 50}, channel, t, nil)
	}
}

func TestMumbleClientsGetAVoiceChannelsChat(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	say5(h, 5, "hello in voice", "second")
	h.UserMoved(7, noChannel, 5)
	got := v.userTexts(7)
	if len(got) != 1 || !strings.Contains(got[0], "Earlier in this channel") || !strings.Contains(got[0], "hello in voice") {
		t.Fatalf("got %q", got)
	}
	// Not where the user may not read (session 2 and channel 9 in the fake).
	say5(h, 9, "secret")
	h.UserMoved(2, noChannel, 9)
	if got := v.userTexts(2); len(got) != 0 {
		t.Fatalf("closed channel: %q", got)
	}
}

func TestWebSessionsGetNoHistoryTexts(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{web: map[uint32]bool{7: true}}
	h.SetVoice(v, "")
	say5(h, 5, "hello")
	h.UserMoved(7, noChannel, 5)
	if got := v.userTexts(7); len(got) != 0 {
		t.Fatalf("web session got %q", got)
	}
}

// Registered Mumble users get what they missed since their last visit, also
// in a new connection (read marks by user id).
func TestRegisteredMumbleUsersGetWhatTheyMissedAcrossConnections(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{users: map[uint32]MumbleUser{7: {Registered: true, UserID: 42}, 8: {Registered: true, UserID: 42}}}
	h.SetVoice(v, "")
	say5(h, 5, "one", "two")
	h.UserMoved(7, noChannel, 5)
	h.UserMoved(7, 5, noChannel) // leaves: read up to "two"
	say5(h, 5, "three")
	h.UserMoved(8, noChannel, 5) // another connection, the same user
	got := v.userTexts(8)
	if len(got) != 1 || !strings.Contains(got[0], "While you were away") || !strings.Contains(got[0], "three") || strings.Contains(got[0], "two") {
		t.Fatalf("got %q", got)
	}
	// Many missed: the newest 20, and how many came before.
	h.UserMoved(8, 5, noChannel)
	for i := range 25 {
		say5(h, 5, fmt.Sprintf("n%d", i))
	}
	v.users[9] = MumbleUser{Registered: true, UserID: 42}
	h.UserMoved(9, noChannel, 5)
	got = v.userTexts(9)
	last := got[len(got)-1]
	if strings.Count(last, "<br>") != 20 || !strings.Contains(last, "n24") || !strings.Contains(last, "5 earlier") {
		t.Fatalf("many missed: %q", last)
	}
}

// The voice server forgets a session before its leave event arrives: the
// read mark still counts.
func TestADisconnectSetsTheReadMark(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{users: map[uint32]MumbleUser{7: {Registered: true, UserID: 42}, 8: {Registered: true, UserID: 42}}, gone: map[uint32]bool{}}
	h.SetVoice(v, "")
	say5(h, 5, "one", "two")
	h.UserMoved(7, noChannel, 5) // stays in the channel, reads live
	say5(h, 5, "three")
	v.gone[7] = true
	h.UserMoved(7, 5, noChannel)
	h.UserMoved(8, noChannel, 5) // next connection: nothing new
	if got := v.userTexts(8); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
	if len(h.visits) != 1 { // only session 8 is remembered
		t.Fatalf("visits %v", h.visits)
	}
}
