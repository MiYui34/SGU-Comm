package sidecar

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// fakeVoice: session 1 may use every channel; session 2 not channel 9.
type fakeVoice struct {
	mu   sync.Mutex
	sent []string
	mods []uint32
	// Text channel mirrors: the last sync, Mumble channels by name, and
	// texts to Mumble clients (channel texts by channel, server texts by
	// session).
	synced    []Mirror
	mirrors   map[string]uint32
	toChannel map[uint32][]string
	toUser    map[uint32][]string
	// Who sessions are: users (others are guests), web (web clients), gone
	// (disconnected: the voice server forgets them before their leave event).
	users map[uint32]MumbleUser
	web   map[uint32]bool
	gone  map[uint32]bool
	// The last SetListenOnly.
	listenOnly []ListenOnlyChannel
	// Registered users' names, and those on a Mumble client (private
	// texts reach them; sent: "from>to:text").
	names  map[uint32]string
	native map[uint32]bool
	sentTo []string
}

func (f *fakeVoice) SendPrivateText(from uint32, _ string, toUser uint32, text string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.native[toUser] {
		return false
	}
	f.sentTo = append(f.sentTo, fmt.Sprintf("%d>%d:%s", from, toUser, text))
	return true
}

func (f *fakeVoice) privateTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sentTo)
}

func (f *fakeVoice) UserNames(ids []uint32) map[uint32]string {
	out := map[uint32]string{}
	for _, id := range ids {
		if n, ok := f.names[id]; ok {
			out[id] = n
		}
	}
	return out
}

func (f *fakeVoice) MumbleClient(session uint32) (MumbleUser, bool) {
	return f.users[session], !f.web[session] && !f.gone[session]
}

func (f *fakeVoice) MayRead(session, channel uint32) bool {
	return !f.gone[session] && (session == 1 || channel != 9)
}
func (f *fakeVoice) MayWrite(session, channel uint32) bool { return session == 1 || channel != 9 }
func (f *fakeVoice) SendChannelText(from, channel uint32, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	if f.toChannel == nil {
		f.toChannel = map[uint32][]string{}
	}
	f.toChannel[channel] = append(f.toChannel[channel], text)
}

// Mirrors get Mumble channels 100, 101, ... by name.
func (f *fakeVoice) SyncTextChannels(want []Mirror) ([]uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mirrors == nil {
		f.mirrors = map[string]uint32{}
	}
	f.synced = slices.Clone(want)
	ids := make([]uint32, len(want))
	for i, m := range want {
		if f.mirrors[m.Name] == 0 {
			f.mirrors[m.Name] = 100 + uint32(len(f.mirrors))
		}
		ids[i] = f.mirrors[m.Name]
	}
	return ids, nil
}

func (f *fakeVoice) SendServerText(session uint32, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.toUser == nil {
		f.toUser = map[uint32][]string{}
	}
	f.toUser[session] = append(f.toUser[session], text)
}

func (f *fakeVoice) userTexts(session uint32) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.toUser[session])
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
	h.MumbleText(mumble.Identity{Session: 3, Name: "native", Registered: true, UserID: 175}, 5, "from mumble", nil)
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

func (f *fakeVoice) MayManage(session uint32) bool { return session == 1 }

// Session 1 is an admin (user id 0).
func (f *fakeVoice) SetModerator(session, user uint32, on bool) error {
	if session != 1 {
		return errors.New("only admins make moderators")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mods = slices.DeleteFunc(f.mods, func(id uint32) bool { return id == user })
	if on {
		f.mods = append(f.mods, user)
	}
	return nil
}

func (f *fakeVoice) Roles() (admins, moderators []uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []uint32{7}, slices.Clone(f.mods)
}

func TestAdminsManageTextChannels(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	admin := readyAs(t, h, 1, "admin")
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"create-channel","name":"games"}`))
	// Errors about channel changes say so (the client shows them apart).
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"create-channel","name":"  Game Night "}`))
	for _, c := range []*Conn{admin, bob} {
		m := next(t, c)
		chans := m["channels"].([]any)
		if m["type"] != "channels" || len(chans) != 2 || chans[1].(map[string]any)["name"] != "game-night" {
			t.Fatalf("got %v", m)
		}
	}
	id := strconv.Itoa(int(next0(t, admin, h)))
	admin.Handle([]byte(`{"type":"rename-channel","channel":` + id + `,"name":"games"}`))
	if m := next(t, admin); m["type"] != "channels" {
		t.Fatalf("rename: %v", m)
	}
	next(t, bob)
	admin.Handle([]byte(`{"type":"order-channels","channels":[` + id + `]}`))
	m := next(t, admin)
	if chans := m["channels"].([]any); chans[0].(map[string]any)["name"] != "games" {
		t.Fatalf("order: %v", m)
	}
	next(t, bob)
	admin.Handle([]byte(`{"type":"delete-channel","channel":` + id + `}`))
	if m := next(t, admin); len(m["channels"].([]any)) != 1 {
		t.Fatalf("delete: %v", m)
	}
	admin.Handle([]byte(`{"type":"create-channel","name":"bad:name"}`))
	if m := next(t, admin); m["type"] != "error" {
		t.Fatalf("bad name: %v", m)
	}
}

// next0 is the id of the second text channel.
func next0(t *testing.T, _ *Conn, h *Hub) float64 {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	return float64(h.channels[1].ID)
}

func TestTextChannelNames(t *testing.T) {
	for in, want := range map[string]string{
		"General":       "general",
		" game  night ": "game-night",
		"Küche":         "küche",
	} {
		if got, ok := textChannelName(in); !ok || got != want {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
	for _, bad := range []string{"", "a:b", strings.Repeat("x", 33), "<b>"} {
		if _, ok := textChannelName(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestModerators(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	admin := h.Connect()
	t.Cleanup(admin.Close)
	admin.SetIdentity(mumble.Identity{Session: 1, Name: "admin"})
	ready := next(t, admin)
	if r, ok := ready["roles"].(map[string]any); !ok || len(r["admins"].([]any)) != 1 || len(r["moderators"].([]any)) != 0 {
		t.Fatalf("ready %v", ready)
	}
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"set-moderator","user":5,"moderator":true}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"set-moderator","user":5,"moderator":true}`))
	// The voice server reports the change; everyone gets the new roles.
	h.RolesChanged()
	for _, c := range []*Conn{admin, bob} {
		if m := next(t, c); m["type"] != "roles" || len(m["moderators"].([]any)) != 1 {
			t.Fatalf("got %v", m)
		}
	}
}

func TestMumbleImagesBecomeAttachments(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	var saved []string
	h.SaveFile = func(author, name, ctype string, data []byte) (store.Upload, error) {
		saved = append(saved, author+":"+name+":"+ctype)
		return h.store.AddUpload(store.Upload{Key: "k" + strconv.Itoa(len(saved)), Name: name, Type: ctype, Size: int64(len(data)), Author: author}, clock)
	}
	web := readyAs(t, h, 2, "web")
	// In the #general mirror (Mumble channel 100 in the fake).
	h.MumbleText(mumble.Identity{Session: 9, Name: "Jukebox", Registered: true, UserID: 814}, 100, "Now playing: x", []Image{{Type: "image/jpeg", Data: []byte("a")}, {Type: "image/png", Data: []byte("b")}})
	first := next(t, web)["message"].(map[string]any)
	second := next(t, web)["message"].(map[string]any)
	if first["text"] != "Now playing: x" || first["upload"].(map[string]any)["type"] != "image/jpeg" ||
		second["text"] != "" || second["upload"].(map[string]any)["type"] != "image/png" {
		t.Fatalf("messages %v %v", first, second)
	}
	if len(saved) != 2 || saved[0] != "Jukebox:image-1.jpg:image/jpeg" {
		t.Fatalf("saved %v", saved)
	}
}

func TestGuestsImagesAreNotStored(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	h.SaveFile = func(string, string, string, []byte) (store.Upload, error) {
		t.Fatal("stored a guest's image")
		return store.Upload{}, nil
	}
	web := readyAs(t, h, 2, "web")
	h.MumbleText(mumble.Identity{Session: 9, Name: "guest", Registered: false}, 100, "look", []Image{{Type: "image/png", Data: []byte("a")}})
	if m := next(t, web)["message"].(map[string]any); m["text"] != "look" || m["upload"] != nil {
		t.Fatalf("message %v", m)
	}
}

// Session 1 may: the fake keeps the last change.
func (f *fakeVoice) SetListenOnly(session, channel uint32, on bool, speakers []uint32) error {
	if session != 1 {
		return errors.New("only admins and moderators make channels listen-only")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listenOnly = nil
	if on {
		f.listenOnly = []ListenOnlyChannel{{Channel: channel, Speakers: []Speaker{}}}
		for _, id := range speakers {
			f.listenOnly[0].Speakers = append(f.listenOnly[0].Speakers, Speaker{ID: id, Name: "user" + strconv.Itoa(int(id))})
		}
	}
	return nil
}

func (f *fakeVoice) ListenOnly() []ListenOnlyChannel {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.listenOnly)
}

func TestListenOnlyChannels(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	mod := h.Connect()
	t.Cleanup(mod.Close)
	mod.SetIdentity(mumble.Identity{Session: 1, Name: "mod"})
	if lo, ok := next(t, mod)["listenOnly"].([]any); !ok || len(lo) != 0 {
		t.Fatal("no listen-only list in ready")
	}
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"set-listen-only","voice":4,"on":true}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	mod.Handle([]byte(`{"type":"set-listen-only","voice":4,"on":true,"users":[9]}`))
	// The voice server reports the change; everyone gets the list.
	h.ListenOnlyChanged()
	for _, c := range []*Conn{mod, bob} {
		m := next(t, c)
		lo := m["channels"].([]any)
		if m["type"] != "listen-only" || len(lo) != 1 {
			t.Fatalf("got %v", m)
		}
		ch := lo[0].(map[string]any)
		sp := ch["speakers"].([]any)[0].(map[string]any)
		if ch["channel"] != float64(4) || sp["id"] != float64(9) || sp["name"] != "user9" {
			t.Fatalf("got %v", m)
		}
	}
}
