package sidecar

import (
	"errors"
	"fmt"
	"testing"
	"time"
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

func TestTimedBansNeedTheBuiltInServer(t *testing.T) {
	h := newHub(t, "general")
	c := ready(t, h, "alice")
	c.Handle([]byte(`{"type":"ban","session":3,"seconds":60}`))
	if m := next(t, c); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}
