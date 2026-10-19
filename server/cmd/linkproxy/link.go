package main

import (
	"sync"
	"time"
)

// link is one direction's bottleneck, shared by all connections.
type link struct {
	mu   sync.Mutex
	free time.Time // when the bytes reserved so far have passed
	bps  float64   // 0: no limit
}

// reserve returns when n bytes, ready at now, have passed the bottleneck.
func (l *link) reserve(n int, now time.Time) time.Time {
	if l.bps <= 0 {
		return now
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	start := now
	if l.free.After(now) {
		start = l.free
	}
	l.free = start.Add(time.Duration(float64(n*8) / l.bps * float64(time.Second)))
	return l.free
}

// jitter is one direction's varying delay, shared by all connections: an
// extra delay up to max, drawn again each period (a link that stalls and
// catches up, as Wi-Fi does). Chunks keep their order.
type jitter struct {
	mu    sync.Mutex
	max   time.Duration // 0: none
	every time.Duration
	rand  func() float64 // in [0, 1)
	extra time.Duration
	drawn time.Time // when extra was drawn
	last  time.Time // the latest arrival so far
}

// at returns when a chunk that passed the bottleneck at end arrives.
func (j *jitter) at(end time.Time, delay time.Duration) time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.max > 0 && (j.drawn.IsZero() || end.Sub(j.drawn) >= j.every) {
		j.extra, j.drawn = time.Duration(j.rand()*float64(j.max)), end
	}
	t := end.Add(delay + j.extra)
	if t.Before(j.last) {
		t = j.last
	}
	j.last = t
	return t
}
