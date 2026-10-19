package main

import (
	"testing"
	"time"
)

func TestReserveQueuesBehindEarlierBytes(t *testing.T) {
	l := &link{bps: 8000} // 1000 bytes/s
	t0 := time.Unix(100, 0)
	if got := l.reserve(500, t0); !got.Equal(t0.Add(500 * time.Millisecond)) {
		t.Fatalf("first: %v", got.Sub(t0))
	}
	// Another connection at once: waits for the first one's bytes.
	if got := l.reserve(250, t0); !got.Equal(t0.Add(750 * time.Millisecond)) {
		t.Fatalf("second: %v", got.Sub(t0))
	}
	// After an idle time the link starts from now.
	later := t0.Add(2 * time.Second)
	if got := l.reserve(1000, later); !got.Equal(later.Add(time.Second)) {
		t.Fatalf("after idle: %v", got.Sub(later))
	}
}

func TestReserveWithoutLimit(t *testing.T) {
	t0 := time.Unix(100, 0)
	if got := (&link{}).reserve(1e6, t0); !got.Equal(t0) {
		t.Fatalf("got %v", got.Sub(t0))
	}
}

func TestJitterKeepsOrderAndChangesEachPeriod(t *testing.T) {
	t0 := time.Unix(100, 0)
	draws := []float64{0.5, 0, 1}
	j := &jitter{max: 100 * time.Millisecond, every: 50 * time.Millisecond, rand: func() float64 { v := draws[0]; draws = draws[1:]; return v }}
	if got := j.at(t0, 20*time.Millisecond); !got.Equal(t0.Add(70 * time.Millisecond)) {
		t.Fatalf("first: %v", got.Sub(t0))
	}
	// Same period: the same extra delay, no new draw.
	if got := j.at(t0.Add(10*time.Millisecond), 20*time.Millisecond); !got.Equal(t0.Add(80 * time.Millisecond)) {
		t.Fatalf("same period: %v", got.Sub(t0))
	}
	// Next period, less delay: never before the chunk ahead (TCP keeps order).
	if got := j.at(t0.Add(60*time.Millisecond), 0); !got.Equal(t0.Add(80 * time.Millisecond)) {
		t.Fatalf("in order: %v", got.Sub(t0))
	}
	if got := j.at(t0.Add(120*time.Millisecond), 0); !got.Equal(t0.Add(220 * time.Millisecond)) {
		t.Fatalf("more delay: %v", got.Sub(t0))
	}
}

func TestNoJitter(t *testing.T) {
	t0 := time.Unix(100, 0)
	if got := (&jitter{}).at(t0, 20*time.Millisecond); !got.Equal(t0.Add(20 * time.Millisecond)) {
		t.Fatalf("got %v", got.Sub(t0))
	}
}
