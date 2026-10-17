package sidecar

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/mumble"
)

// Session 1 may ban; the fake keeps the bans ("session:reason:duration").
func (f *fakeVoice) Ban(actor, session uint32, reason string, d time.Duration) error {
	if actor != 1 {
		return errors.New("you may not ban this user")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bans = append(f.bans, fmt.Sprintf("%d:%s:%v", session, reason, d))
	return nil
}

func TestTimedBans(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	mod := readyAs(t, h, 1, "mod")
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"ban","session":3,"seconds":3600}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	mod.Handle([]byte(`{"type":"ban","session":3,"seconds":-1}`))
	if m := next(t, mod); m["type"] != "error" {
		t.Fatalf("negative duration: %v", m)
	}
	mod.Handle([]byte(`{"type":"ban","session":3,"seconds":3600,"text":"spam"}`))
	mod.Handle([]byte(`{"type":"ban","session":4}`))
	v.mu.Lock()
	defer v.mu.Unlock()
	if fmt.Sprint(v.bans) != "[3:spam:1h0m0s 4::0s]" {
		t.Fatalf("bans %v", v.bans)
	}
}

// Session 1 may time out; the fake keeps them by session.
func (f *fakeVoice) SetTimeout(actor, session uint32, d time.Duration) error {
	if actor != 1 {
		return errors.New("you may not time out this user")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.timeouts == nil {
		f.timeouts = map[uint32]time.Time{}
	}
	delete(f.timeouts, session)
	if d > 0 {
		f.timeouts[session] = clock.Add(d)
	}
	return nil
}

func (f *fakeVoice) TimedOut(session uint32) (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	until, ok := f.timeouts[session]
	return until, ok
}

// Timeouts: user 7's, and the guests' ones.
func (f *fakeVoice) Timeouts() []Timeout {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Timeout{{Registered: true, UserID: 7, Until: clock.Add(time.Hour)}}
	for s, until := range f.timeouts {
		out = append(out, Timeout{Session: s, Until: until})
	}
	return out
}

func TestTimeOuts(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	mod := h.Connect()
	t.Cleanup(mod.Close)
	mod.SetIdentity(mumble.Identity{Session: 1, Name: "mod"})
	ms := func(d time.Duration) float64 { return float64(clock.Add(d).UnixMilli()) }
	if list, _ := next(t, mod)["timeouts"].([]any); len(list) != 1 || fmt.Sprint(list[0]) != fmt.Sprint(map[string]any{"user": 7.0, "until": ms(time.Hour)}) {
		t.Fatalf("ready %v", list)
	}
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"time-out","session":3,"seconds":300}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	mod.Handle([]byte(`{"type":"time-out","session":2,"seconds":300}`))
	// The voice server reports the change; everyone gets the list.
	h.TimeoutsChanged()
	for _, c := range []*Conn{mod, bob} {
		m := next(t, c)
		if list, _ := m["timeouts"].([]any); m["type"] != "timeouts" || len(list) != 2 ||
			fmt.Sprint(list[1]) != fmt.Sprint(map[string]any{"session": 2.0, "until": ms(5 * time.Minute)}) {
			t.Fatalf("got %v", m)
		}
	}
	// Timed out: no sending, reacting or editing.
	for _, req := range []string{
		`{"type":"send","channel":1,"text":"hi"}`,
		`{"type":"react","message":1,"emoji":"x","on":true}`,
		`{"type":"edit-message","message":1,"text":"hi"}`,
	} {
		bob.Handle([]byte(req))
		if m := next(t, bob); m["type"] != "error" || !strings.Contains(m["message"].(string), "timed out") {
			t.Fatalf("%s: %v", req, m)
		}
	}
	mod.Handle([]byte(`{"type":"send","channel":1,"text":"hi"}`))
	if m := next(t, mod); m["type"] != "message" {
		t.Fatalf("mod got %v", m)
	}
}

func TestTimedBansNeedTheBuiltInServer(t *testing.T) {
	h := newHub(t, "general")
	c := ready(t, h, "alice")
	c.Handle([]byte(`{"type":"ban","session":3,"seconds":60}`))
	if m := next(t, c); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}
