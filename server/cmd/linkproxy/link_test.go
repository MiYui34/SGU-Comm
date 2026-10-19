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
