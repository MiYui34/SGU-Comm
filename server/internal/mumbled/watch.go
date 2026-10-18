package mumbled

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

// Connection watch: every 10 s the server keeps a snapshot of each
// connection's counters, the last three of them. The admin view
// (Connections) and the log count the recent window against the oldest
// one: 30 to 40 s, not since the connection started.

const (
	watchEvery     = 10 * time.Second
	watchSnapshots = 3
	// Poor this long: logged; good this long again: logged again. Fair
	// changes nothing, so a connection near a limit does not flood the log.
	watchAfter = 30 * time.Second
	// Fewer voice frames in the window give no percentage (1 s of 20 ms
	// packets).
	minFrames = 50
)

// Connection is one connection over the recent window, for the admin view:
// no address (admins see it in Mumble's user information).
type Connection struct {
	Session         uint32
	Name, Transport string
	// The client's ping in ms over its voice transport (nil: none
	// measured), and late or lost voice in percent, from the client and
	// to it (nil: too little voice).
	Ping, FromClient, FromServer *float64
	Dropped, TextOverflows       uint64 // in the window
	Online                       int64  // seconds
	Client, OS                   string
}

type quality int

const (
	unknown quality = iota
	good
	fair
	poor
)

// level as the web client's connectionQuality: good below 100 ms and 2 %,
// fair below 250 ms and 8 %, else poor (the worse of both).
func (c Connection) level() quality {
	if c.Ping == nil && c.FromClient == nil && c.FromServer == nil {
		return unknown
	}
	ping, bad := deref(c.Ping), max(deref(c.FromClient), deref(c.FromServer))
	switch {
	case ping < 100 && bad < 2:
		return good
	case ping < 250 && bad < 8:
		return fair
	}
	return poor
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// sample is a connection's counters at one tick.
type sample struct {
	up, down           VoiceStats
	dropped, overflows uint64
}

func sampleOf(st ConnStats) sample {
	return sample{st.UDP.plus(st.TCP), st.FromServer, st.Dropped, st.TextOverflows}
}

// connection: st against an older sample (zero: since it started).
func connection(st ConnStats, old sample) Connection {
	now := sampleOf(st)
	c := Connection{
		Session: st.Session, Name: st.Name, Transport: st.Transport,
		FromClient: badPercent(now.up, old.up), FromServer: badPercent(now.down, old.down),
		Dropped: since(now.dropped, old.dropped), TextOverflows: since(now.overflows, old.overflows),
		Online: int64(st.Online / time.Second), Client: st.Release, OS: strings.TrimSpace(st.OS + " " + st.OSVersion),
	}
	if c.Client == "" && st.Version != 0 {
		c.Client = fmt.Sprintf("%d.%d.%d", st.Version>>48, st.Version>>32&0xffff, st.Version>>16&0xffff)
	}
	if st.Transport == "udp" && st.UDPPackets > 0 {
		c.Ping = ptr(float64(st.UDPPing.Avg))
	} else if st.TCPPackets > 0 {
		c.Ping = ptr(float64(st.TCPPing.Avg))
	}
	return c
}

func ptr[T any](v T) *T { return &v }

// since: a counter's growth (a client that starts its own over: all of it).
func since[T uint32 | uint64](now, old T) T {
	if now < old {
		return now
	}
	return now - old
}

// badPercent: late and lost frames between two counts. Good counts late
// frames too, as in the crypt; the web client's reports do not, which
// overstates them a little (8.7 % instead of 8 %).
func badPercent(now, old VoiceStats) *float64 {
	good, late, lost := since(now.Good, old.Good), since(now.Late, old.Late), since(now.Lost, old.Lost)
	if good+lost < minFrames {
		return nil
	}
	return ptr(float64(late+lost) / float64(good+lost) * 100)
}

type watch struct {
	mu    sync.Mutex
	conns map[uint32]*watched
}

type watched struct {
	samples              []sample // oldest first
	poorSince, goodSince time.Time
	bad                  bool // logged as poor
}

func (w *watch) oldest(session uint32) sample {
	if x := w.conns[session]; x != nil && len(x.samples) > 0 {
		return x.samples[0]
	}
	return sample{}
}

// connections over the window.
func (w *watch) connections(stats []ConnStats) []Connection {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Connection, len(stats))
	for i, st := range stats {
		out[i] = connection(st, w.oldest(st.Session))
	}
	return out
}

// tick takes a snapshot of every connection and logs those that turned
// poor or good again.
func (w *watch) tick(now time.Time, stats []ConnStats, log *slog.Logger) {
	w.mu.Lock()
	defer w.mu.Unlock()
	conns := make(map[uint32]*watched, len(stats))
	for _, st := range stats {
		c := connection(st, w.oldest(st.Session))
		x := w.conns[st.Session]
		if x == nil {
			x = &watched{}
		}
		conns[st.Session] = x
		x.samples = append(x.samples, sampleOf(st))
		x.samples = x.samples[max(len(x.samples)-watchSnapshots, 0):]
		switch c.level() {
		case poor:
			x.goodSince = time.Time{}
			x.poorSince = firstTime(x.poorSince, now)
		case good:
			x.poorSince = time.Time{}
			x.goodSince = firstTime(x.goodSince, now)
		default:
			x.poorSince, x.goodSince = time.Time{}, time.Time{}
		}
		if !x.bad && !x.poorSince.IsZero() && now.Sub(x.poorSince) >= watchAfter {
			x.bad = true
			log.Warn("connection poor", connAttrs(c)...)
		} else if x.bad && !x.goodSince.IsZero() && now.Sub(x.goodSince) >= watchAfter {
			x.bad = false
			log.Info("connection good again", connAttrs(c)...)
		}
	}
	w.conns = conns
}

func firstTime(t, now time.Time) time.Time {
	if t.IsZero() {
		return now
	}
	return t
}

func connAttrs(c Connection) []any {
	return []any{"name", c.Name, "session", c.Session, "transport", c.Transport,
		"ping", rounded(c.Ping), "late_lost_from_client", rounded(c.FromClient), "late_lost_to_client", rounded(c.FromServer)}
}

// rounded to a tenth, for the log ("-": none).
func rounded(p *float64) any {
	if p == nil {
		return "-"
	}
	return math.Round(*p*10) / 10
}

// Watch checks the connections every 10 s until done is closed.
func (s *Server) Watch(done <-chan struct{}) {
	t := time.NewTicker(watchEvery)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-t.C:
			s.watch.tick(now, s.ConnectionStats(), slog.Default())
		}
	}
}

// Connections lists the connections over the recent window (admins only).
func (s *Server) Connections(actor uint32) ([]Connection, error) {
	s.mu.Lock()
	u := s.state.Users[actor]
	admin := u != nil && s.isAdmin(u)
	s.mu.Unlock()
	if !admin {
		return nil, errors.New("only admins see the connections")
	}
	return s.watch.connections(s.ConnectionStats()), nil
}
