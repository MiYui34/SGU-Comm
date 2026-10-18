package sidecar

import (
	"cmp"
	"errors"
	"testing"

	"kallisti.chat/server/internal/mumble"
)

// Session 1 is an admin.
func (f *fakeVoice) SetCaptcha(session uint32, level string) error {
	if session != 1 {
		return errors.New("only admins change this setting")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.captcha = level
	return nil
}

func (f *fakeVoice) Captcha() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cmp.Or(f.captcha, "off")
}

func TestCaptcha(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	admin := readyAs(t, h, 1, "admin")
	bob := readyAs(t, h, 2, "bob")

	bob.Handle([]byte(`{"type":"set-captcha","level":"hard"}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" || v.Captcha() != "off" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"set-captcha","level":"hard"}`))
	for _, c := range []*Conn{admin, bob} {
		if m := next(t, c); m["type"] != "captcha" || m["level"] != "hard" {
			t.Fatalf("got %v", m)
		}
	}
	late := h.Connect()
	t.Cleanup(late.Close)
	late.SetIdentity(mumble.Identity{Session: 4, Name: "late"})
	if m := next(t, late); m["captcha"] != "hard" {
		t.Fatalf("ready %v", m)
	}
}
