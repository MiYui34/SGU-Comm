package sidecar

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/mumble"
)

// Staff (session 1) moderate everywhere; others where f.moderates says
// (session: Mumble channel).
func (f *fakeVoice) ModeratesIn(session, channel uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return session == 1 || channel != 0 && f.moderates[session] == channel
}

func (f *fakeVoice) ChannelModerators() []ChannelModerators {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ChannelModerators{}, f.channelMods...)
}

func (f *fakeVoice) SetChannelModerator(session, user, channel uint32, on bool) error {
	if session != 1 {
		return errors.New("only admins and moderators make channel moderators")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channelMods = append(f.channelMods, ChannelModerators{Channel: channel, Users: []uint32{user}})
	return nil
}

// Channel time-outs by session and Mumble channel (no subchannels here).
func (f *fakeVoice) SetChannelTimeout(actor, session, channel uint32, d time.Duration) error {
	if !f.ModeratesIn(actor, channel) {
		return errors.New("you may not time out this user")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chanTimeouts == nil {
		f.chanTimeouts = map[[2]uint32]time.Time{}
	}
	delete(f.chanTimeouts, [2]uint32{session, channel})
	if d > 0 {
		f.chanTimeouts[[2]uint32{session, channel}] = clock.Add(d)
	}
	return nil
}

func (f *fakeVoice) ChannelTimedOut(session, channel uint32) (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	until, ok := f.chanTimeouts[[2]uint32{session, channel}]
	return until, ok
}

func (f *fakeVoice) ChannelTimeouts() []Timeout {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Timeout
	for k, until := range f.chanTimeouts {
		out = append(out, Timeout{Session: k[0], Channel: k[1], Until: until})
	}
	return out
}

// nextOf reads a connection's messages up to one of a type.
func nextOf(t *testing.T, c *Conn, typ string) map[string]any {
	t.Helper()
	for {
		if m := next(t, c); m["type"] == typ {
			return m
		}
	}
}

func (f *fakeVoice) setModerates(session, channel uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.moderates[session] = channel
}

// modHub has the text channels general (mirror 100) and other (mirror
// 101); carol (session 3) moderates general and voice channel 5, bob
// (session 2) is a plain user.
func modHub(t *testing.T) (h *Hub, v *fakeVoice, carol, bob *Conn) {
	t.Helper()
	h = newHub(t, "general", "other")
	v = &fakeVoice{moderates: map[uint32]uint32{3: 100}}
	h.SetVoice(v, "")
	carol = joinAs(t, h, mumble.Identity{Session: 3, Name: "carol", Registered: true, UserID: 30})
	bob = joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 20})
	return h, v, carol, bob
}

// postIn sends a text to a store channel and returns the message id (the
// others' copies are read).
func postIn(t *testing.T, from *Conn, channel int64, text string, others ...*Conn) string {
	t.Helper()
	from.Handle([]byte(`{"type":"send","channel":` + strconv.FormatInt(channel, 10) + `,"text":"` + text + `"}`))
	m := next(t, from)
	if m["type"] != "message" {
		t.Fatalf("send: %v", m)
	}
	for _, o := range others {
		next(t, o)
	}
	return strconv.FormatInt(int64(m["message"].(map[string]any)["id"].(float64)), 10)
}

func TestChannelModeratorsDeleteAndPinInTheirChannel(t *testing.T) {
	h, _, carol, bob := modHub(t)
	inGeneral := postIn(t, bob, h.channels[0].ID, "spam", carol)
	inOther := postIn(t, bob, h.channels[1].ID, "spam", carol)
	for _, req := range []string{
		`{"type":"pin","message":` + inOther + `,"on":true}`,
		`{"type":"delete-message","message":` + inOther + `}`,
	} {
		carol.Handle([]byte(req))
		if m := next(t, carol); m["type"] != "error" {
			t.Fatalf("%s: %v", req, m)
		}
	}
	carol.Handle([]byte(`{"type":"pin","message":` + inGeneral + `,"on":true}`))
	if m := next(t, carol); m["type"] != "message-pinned" {
		t.Fatalf("pin: %v", m)
	}
	next(t, bob)
	carol.Handle([]byte(`{"type":"delete-message","message":` + inGeneral + `}`))
	if m := next(t, carol); m["type"] != "message-deleted" {
		t.Fatalf("delete: %v", m)
	}
	next(t, bob)
	// In a voice channel's chat too.
	five, _ := h.store.VoiceChat(5)
	six, _ := h.store.VoiceChat(6)
	inFive := postIn(t, bob, five, "spam", carol)
	inSix := postIn(t, bob, six, "spam", carol)
	carol.Handle([]byte(`{"type":"delete-message","message":` + inSix + `}`))
	if m := next(t, carol); m["type"] != "error" {
		t.Fatalf("delete in 6: %v", m)
	}
	h.voice.(*fakeVoice).setModerates(3, 5)
	carol.Handle([]byte(`{"type":"delete-message","message":` + inFive + `}`))
	if m := next(t, carol); m["type"] != "message-deleted" {
		t.Fatalf("delete in 5: %v", m)
	}
}

func TestChannelModeratorsSetTopicAndSlowMode(t *testing.T) {
	h, _, carol, bob := modHub(t)
	general, other := strconv.FormatInt(h.channels[0].ID, 10), strconv.FormatInt(h.channels[1].ID, 10)
	for _, req := range []string{
		`{"type":"set-topic","channel":` + other + `,"text":"hi"}`,
		`{"type":"set-slow-mode","channel":` + other + `,"seconds":30}`,
		`{"type":"rename-channel","channel":` + general + `,"name":"mine"}`,
		`{"type":"delete-channel","channel":` + general + `}`,
	} {
		carol.Handle([]byte(req))
		if m := next(t, carol); m["type"] != "error" || m["about"] != "manage" {
			t.Fatalf("%s: %v", req, m)
		}
	}
	carol.Handle([]byte(`{"type":"set-topic","channel":` + general + `,"text":"be nice"}`))
	carol.Handle([]byte(`{"type":"set-slow-mode","channel":` + general + `,"seconds":30}`))
	for range 2 {
		next(t, bob)
		next(t, carol)
	}
	if h.channels[0].Topic != "be nice" || h.channels[0].SlowMode != 30 {
		t.Fatalf("channel %+v", h.channels[0])
	}
	// Slow mode holds bob there, not its moderator.
	postIn(t, bob, h.channels[0].ID, "one", carol)
	postIn(t, carol, h.channels[0].ID, "one", bob)
	postIn(t, carol, h.channels[0].ID, "two", bob)
	bob.Handle([]byte(`{"type":"send","channel":` + general + `,"text":"two"}`))
	if m := next(t, bob); m["about"] != "slow-mode" {
		t.Fatalf("bob got %v", m)
	}
}

func TestChannelModeratorsManagePrivateInvites(t *testing.T) {
	h, v, _ := privateHub(t)
	v.moderates = map[uint32]uint32{3: 101}
	carol := joinAs(t, h, mumble.Identity{Session: 3, Name: "carol", Registered: true, UserID: 30})
	carol.Handle([]byte(`{"type":"create-invite","voice":101}`))
	if m := next(t, carol); m["type"] != "invite-created" {
		t.Fatalf("got %v", m)
	}
	m := next(t, carol)
	if m["type"] != "invites" {
		t.Fatalf("got %v", m)
	}
	invite := strconv.FormatInt(int64(m["invites"].([]any)[0].(map[string]any)["id"].(float64)), 10)
	// Members are shown to the channel's moderators.
	if p := h.privateFor(mumble.Identity{Session: 3, Registered: true, UserID: 30}); len(p) != 1 || len(p[0].Members) == 0 {
		t.Fatalf("private %+v", p)
	}
	// Not another channel's links.
	v.setModerates(3, 100)
	carol.Handle([]byte(`{"type":"delete-invite","invite":` + invite + `}`))
	if m := next(t, carol); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
	v.setModerates(3, 101)
	carol.Handle([]byte(`{"type":"delete-invite","invite":` + invite + `}`))
	if m := next(t, carol); m["type"] != "invites" || len(m["invites"].([]any)) != 0 {
		t.Fatalf("got %v", m)
	}
}

func TestStaffAppointChannelModerators(t *testing.T) {
	h, v, carol, bob := modHub(t)
	carol.Handle([]byte(`{"type":"set-channel-moderator","user":20,"voice":100,"on":true}`))
	if m := next(t, carol); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("carol got %v", m)
	}
	staff := readyAs(t, h, 1, "staff")
	staff.Handle([]byte(`{"type":"set-channel-moderator","user":20,"voice":100,"on":true}`))
	h.RolesChanged()
	for _, c := range []*Conn{staff, bob} {
		if m := nextOf(t, c, "roles"); fmt.Sprint(m["channelModerators"]) != "[map[channel:100 users:[20]]]" {
			t.Fatalf("roles %v", m)
		}
	}
	// Only those who see the channel learn its moderators.
	v.mu.Lock()
	v.hidden = map[uint32]bool{100: true}
	v.mu.Unlock()
	h.RolesChanged()
	if m := nextOf(t, bob, "roles"); fmt.Sprint(m["channelModerators"]) != "[]" {
		t.Fatalf("bob got %v", m)
	}
}

func TestChannelTimeOuts(t *testing.T) {
	h, _, carol, bob := modHub(t)
	general := strconv.FormatInt(h.channels[0].ID, 10)
	msg := postIn(t, bob, h.channels[0].ID, "hi", carol)
	carol.Handle([]byte(`{"type":"channel-time-out","session":2,"voice":101,"seconds":300}`))
	if m := next(t, carol); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("other channel: %v", m)
	}
	carol.Handle([]byte(`{"type":"channel-time-out","session":2,"voice":100,"seconds":300}`))
	h.TimeoutsChanged()
	m := next(t, bob)
	if list, _ := m["channelTimeouts"].([]any); m["type"] != "timeouts" || len(list) != 1 ||
		fmt.Sprint(list[0]) != fmt.Sprint(map[string]any{"session": 2.0, "channel": 100.0, "until": float64(clock.Add(5 * time.Minute).UnixMilli())}) {
		t.Fatalf("bob got %v", m)
	}
	next(t, carol)
	for _, req := range []string{
		`{"type":"send","channel":` + general + `,"text":"hi"}`,
		`{"type":"react","message":` + msg + `,"emoji":"x","on":true}`,
		`{"type":"edit-message","message":` + msg + `,"text":"hey"}`,
	} {
		bob.Handle([]byte(req))
		if m := next(t, bob); m["type"] != "error" || !strings.Contains(m["message"].(string), "timed out in this channel") {
			t.Fatalf("%s: %v", req, m)
		}
	}
	postIn(t, bob, h.channels[1].ID, "elsewhere", carol)
}
