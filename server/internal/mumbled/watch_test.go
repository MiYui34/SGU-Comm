package mumbled

import (
	"bytes"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"
)

func TestConnectionsAreForAdmins(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	alice := dial(t, addr, "alice")
	if _, err := s.Connections(alice.session); err == nil {
		t.Fatal("a user got the connections")
	}
	conns, err := s.Connections(su.session)
	if err != nil || len(conns) != 2 || conns[1].Name != "alice" || conns[1].Transport != "tcp" || conns[1].Client != "test" {
		t.Fatalf("connections %+v, %v", conns, err)
	}
}

// A Mumble client over UDP: what it sent (up), what it got (down).
func udpStats(online time.Duration, up, down VoiceStats, ping float32) ConnStats {
	return ConnStats{
		Session: 5, Name: "alice", Transport: "udp", Online: online, UDP: up, OS: "Linux", OSVersion: "6.1", Version: 1<<48 | 5<<32 | 634<<16,
		Reported: Reported{FromServer: down, UDPPing: Ping{Avg: ping}, UDPPackets: 3, TCPPing: Ping{Avg: 999}, TCPPackets: 3},
	}
}

func near(p *float64, want float64) bool { return p != nil && math.Abs(*p-want) < 1e-9 }

func TestConnectionsCountTheRecentWindow(t *testing.T) {
	var w watch
	t0 := time.Unix(1000, 0)
	log := slog.New(slog.DiscardHandler)
	// Ten seconds apart: 1000 frames each (late ones are also good, as in
	// the crypt).
	for i := range 4 {
		up := VoiceStats{Good: uint32(1000 * (i + 1)), Late: uint32(10 * i)}
		down := VoiceStats{Good: uint32(1000 * (i + 1)), Lost: uint32(50 * max(i-1, 0))}
		st := udpStats(time.Duration(i)*watchEvery, up, down, 40)
		st.Dropped = uint64(7 * i)
		w.tick(t0.Add(time.Duration(i)*watchEvery), []ConnStats{st}, log)
	}
	// Now against the oldest kept snapshot (10 s): 20 late of 2000 up,
	// 100 lost of 2100 down.
	st := udpStats(35*time.Second, VoiceStats{Good: 4000, Late: 30}, VoiceStats{Good: 4000, Lost: 100}, 40)
	st.Dropped = 30
	c := w.connections([]ConnStats{st})[0]
	if *c.Ping != 40 || !near(c.FromClient, 1) || !near(c.FromServer, 100.0/2100*100) || c.Dropped != 23 || c.Online != 35 ||
		c.Client != "1.5.634" || c.OS != "Linux 6.1" {
		t.Fatalf("connection %+v", c)
	}
	// Too little voice for a percentage; no ping measured yet; a new
	// connection counts since it started.
	quiet := ConnStats{Session: 6, Transport: "websocket", TCP: VoiceStats{Good: 10, Lost: 10}, Release: "Kallisti web"}
	c = w.connections([]ConnStats{quiet})[0]
	if c.Ping != nil || c.FromClient != nil || c.FromServer != nil || c.Client != "Kallisti web" {
		t.Fatalf("quiet %+v", c)
	}
	// Web clients report their TCP ping.
	quiet.TCPPing, quiet.TCPPackets = Ping{Avg: 80}, 2
	if c = w.connections([]ConnStats{quiet})[0]; *c.Ping != 80 {
		t.Fatalf("web ping %+v", c)
	}
	// Gone sessions are forgotten at the next tick.
	w.tick(t0.Add(time.Minute), nil, log)
	if len(w.conns) != 0 {
		t.Fatalf("kept %v", w.conns)
	}
}

func TestBadConnectionsAreLoggedOnce(t *testing.T) {
	var w watch
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	t0 := time.Unix(1000, 0)
	at := 0
	run := func(ping float32, ticks int) {
		for range ticks {
			w.tick(t0.Add(time.Duration(at)*watchEvery), []ConnStats{udpStats(time.Duration(at)*watchEvery, VoiceStats{}, VoiceStats{}, ping)}, log)
			at++
		}
	}
	lines := func() []string { return strings.Split(strings.TrimSpace(buf.String()), "\n") }

	run(300, 3) // poor for 20 s
	run(150, 1) // fair: starts over
	run(300, 3)
	if buf.Len() != 0 {
		t.Fatalf("logged %q", buf.String())
	}
	run(300, 5) // poor for 30 s: once
	if l := lines(); len(l) != 1 || !strings.Contains(l[0], "connection poor") || !strings.Contains(l[0], "name=alice") ||
		!strings.Contains(l[0], "session=5") || !strings.Contains(l[0], "ping=300") {
		t.Fatalf("logged %q", l)
	}
	run(150, 5) // fair: still bad
	run(50, 3)  // good for 20 s
	run(300, 1)
	run(50, 3)
	if l := lines(); len(l) != 1 {
		t.Fatalf("logged %q", l)
	}
	run(50, 1) // good for 30 s
	if l := lines(); len(l) != 2 || !strings.Contains(l[1], "connection good again") {
		t.Fatalf("logged %q", l)
	}
	run(50, 5)
	if l := lines(); len(l) != 2 {
		t.Fatalf("logged %q", l)
	}
}
