package sidecar

import (
	"strconv"
	"sync"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

// fakeVoice: session 1 may use every channel; session 2 not channel 9.
type fakeVoice struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeVoice) MayRead(session, channel uint32) bool  { return session == 1 || channel != 9 }
func (f *fakeVoice) MayWrite(session, channel uint32) bool { return session == 1 || channel != 9 }
func (f *fakeVoice) SendChannelText(from, channel uint32, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
}

func readyAs(t *testing.T, h *Hub, session uint32, name string) *Conn {
	t.Helper()
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: session, Name: name})
	if m := next(t, c); m["type"] != "ready" || m["voiceChats"] != true {
		t.Fatalf("got %v", m)
	}
	return c
}

// voiceChat asks for the stored chat of a voice channel.
func voiceChat(t *testing.T, c *Conn, voice int) float64 {
	t.Helper()
	c.Handle([]byte(`{"type":"voice-chat","voice":` + strconv.Itoa(voice) + `}`))
	m := next(t, c)
	if m["type"] != "voice-chat" || m["voice"] != float64(voice) {
		t.Fatalf("got %v", m)
	}
	return m["channel"].(float64)
}


func TestVoiceChannelChats(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "https://chat.example")
	alice := readyAs(t, h, 1, "alice")
	bob := readyAs(t, h, 2, "bob")
	id := voiceChat(t, alice, 5)
	if again := voiceChat(t, bob, 5); again != id {
		t.Fatalf("two chats for one channel: %v %v", id, again)
	}
	// A message goes to the readers and to Mumble clients.
	alice.Handle([]byte(`{"type":"send","channel":` + strconv.Itoa(int(id)) + `,"text":"hello"}`))
	if m := next(t, alice); m["type"] != "message" {
		t.Fatalf("alice got %v", m)
	}
	if m := next(t, bob); m["type"] != "message" {
		t.Fatalf("bob got %v", m)
	}
	v.mu.Lock()
	if len(v.sent) != 1 || v.sent[0] != "hello" {
		t.Fatalf("to Mumble: %v", v.sent)
	}
	v.mu.Unlock()
	// Text from Mumble clients is stored and shown.
	h.MumbleText(3, "native", true, 5, "from mumble")
	if m := next(t, bob); m["type"] != "message" || m["message"].(map[string]any)["author"] != "native" {
		t.Fatalf("bob got %v", m)
	}
	next(t, alice)
	// History.
	bob.Handle([]byte(`{"type":"history","channel":` + strconv.Itoa(int(id)) + `}`))
	if m := next(t, bob); m["type"] != "history" || len(m["messages"].([]any)) != 2 {
		t.Fatalf("history %v", m)
	}
}

func TestClosedVoiceChatsStayClosed(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	alice := readyAs(t, h, 1, "alice")
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"voice-chat","voice":9}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("bob got %v", m)
	}
	id := voiceChat(t, alice, 9)
	// Bob cannot read it by its channel id either, nor gets its messages.
	bob.Handle([]byte(`{"type":"history","channel":` + strconv.Itoa(int(id)) + `}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("bob got %v", m)
	}
	alice.Handle([]byte(`{"type":"send","channel":` + strconv.Itoa(int(id)) + `,"text":"secret"}`))
	next(t, alice)
	empty(t, bob)
	bob.Handle([]byte(`{"type":"send","channel":` + strconv.Itoa(int(id)) + `,"text":"let me in"}`))
	if m := next(t, bob); m["type"] != "error" {
		t.Fatalf("bob got %v", m)
	}
}

func TestVoiceChatsNeedTheBuiltInServer(t *testing.T) {
	h := newHub(t, "general")
	c := ready(t, h, "alice")
	c.Handle([]byte(`{"type":"voice-chat","voice":5}`))
	if m := next(t, c); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}
