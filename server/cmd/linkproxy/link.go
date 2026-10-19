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
