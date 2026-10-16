package main

import (
	"testing"
	"time"
)

// spurt makes packets with frame numbers from first, step 2 (20 ms), 20 ms
// apart, the last one a terminator.
func makeSpurt(at time.Time, first uint64, n int) []packet {
	var ps []packet
	for i := range n {
		ps = append(ps, packet{frame: first + uint64(2*i), at: at.Add(time.Duration(i) * 20 * time.Millisecond), terminator: i == n-1})
	}
	return ps
}

func TestCleanSpurtsPass(t *testing.T) {
	t0 := time.Unix(0, 0)
	ps := append(makeSpurt(t0, 0, 50), makeSpurt(t0.Add(3*time.Second), 100, 30)...)
	r := check(ps, ps[len(ps)-1].at.Add(time.Second))
	if len(r.spurts) != 2 || !r.ok() {
		t.Fatalf("report %+v", r)
	}
}

func TestProblemsAreFound(t *testing.T) {
	t0 := time.Unix(0, 0)
	ps := makeSpurt(t0, 0, 10)
	ps = append(ps[:4], append([]packet{ps[3]}, ps[4:]...)...) // a repeat
	ps = append(ps[:7], ps[8:]...)                             // a gap
	// A packet after the terminator, with a repeated frame number (Chrome's
	// flush() did this).
	last := ps[len(ps)-1]
	ps = append(ps, packet{frame: last.frame, at: last.at.Add(5 * time.Millisecond)})
	// A spurt without a terminator.
	tail := makeSpurt(t0.Add(3*time.Second), 100, 5)
	tail[4].terminator = false
	ps = append(ps, tail...)
	r := check(ps, ps[len(ps)-1].at.Add(time.Second))
	s := r.spurts
	if len(s) != 3 || s[0].repeats != 1 || s[0].gaps != 1 || s[0].terminators != 1 {
		t.Fatalf("first spurt %+v", s)
	}
	if s[1].packets != 1 || s[1].terminators != 0 || s[1].afterTerminator != true {
		t.Fatalf("after the terminator %+v", s[1])
	}
	if s[2].terminators != 0 || r.ok() {
		t.Fatalf("report %+v", r)
	}
}

func TestParseVoice(t *testing.T) {
	// Opus, session 5, frame 256, header 3 bytes with the terminator bit.
	b := []byte{0x80, 0x05, 0x81, 0x00, 0xa0, 0x03, 1, 2, 3}
	s, p, ok := parseVoice(b)
	if !ok || s != 5 || p.frame != 256 || !p.terminator {
		t.Fatalf("got %d %+v %v", s, p, ok)
	}
	if _, _, ok := parseVoice([]byte{0x20, 0x05}); ok {
		t.Fatal("a ping parsed as voice")
	}
}

func TestASpurtRunningAtTheEndIsOpen(t *testing.T) {
	ps := makeSpurt(time.Unix(0, 0), 0, 10)
	ps[9].terminator = false
	if r := check(ps, ps[9].at.Add(50*time.Millisecond)); !r.ok() || !r.spurts[0].open {
		t.Fatalf("report %+v", r)
	}
}
