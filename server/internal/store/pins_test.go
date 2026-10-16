package store

import (
	"errors"
	"testing"
	"time"
)

func TestPins(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	cs, _ := s.Channels()
	ch := cs[0].ID
	t0 := time.UnixMilli(1_000_000).UTC()
	a, _ := s.AddMessage(NewMessage{ChannelID: ch, Author: "a", Text: "first", At: t0})
	b, _ := s.AddMessage(NewMessage{ChannelID: ch, Author: "b", Text: "second", At: t0})

	m, changed, err := s.SetPinned(a.ID, t0.Add(time.Minute))
	if err != nil || !changed || !m.PinnedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("pin: %+v %v %v", m, changed, err)
	}
	if _, changed, _ := s.SetPinned(a.ID, t0.Add(2*time.Minute)); changed {
		t.Fatal("pinned twice")
	}
	s.SetPinned(b.ID, t0.Add(3*time.Minute))
	pins, _ := s.Pinned(ch)
	if len(pins) != 2 || pins[0].ID != b.ID || pins[1].ID != a.ID {
		t.Fatalf("pins (newest pin first) %+v", pins)
	}
	// History shows them too.
	if h, _ := s.History(ch, 0, 10); h[0].PinnedAt.IsZero() {
		t.Fatal("history without the pin")
	}
	m, changed, _ = s.SetPinned(a.ID, time.Time{})
	if !changed || !m.PinnedAt.IsZero() {
		t.Fatalf("unpin: %+v %v", m, changed)
	}
	if _, _, err := s.SetPinned(9999, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown message: %v", err)
	}
}

func TestPinsHaveALimit(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	cs, _ := s.Channels()
	t0 := time.UnixMilli(1_000_000).UTC()
	var last Message
	for range MaxPins + 1 {
		last, _ = s.AddMessage(NewMessage{ChannelID: cs[0].ID, Author: "a", Text: "x", At: t0})
		if _, _, err := s.SetPinned(last.ID, t0); err != nil && last.ID <= MaxPins {
			t.Fatal(err)
		}
	}
	if _, _, err := s.SetPinned(last.ID, t0); !errors.Is(err, ErrTooManyPins) {
		t.Fatalf("pin %d: %v", MaxPins+1, err)
	}
}
