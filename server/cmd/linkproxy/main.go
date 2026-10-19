// linkproxy is a TCP proxy that models a slow link for the perf tests
// (tests/perf): all connections share one bottleneck per direction
// (bits/s), plus a one-way delay. Browsers' own throttling is per
// request, so it misses the competition between the socket and HTTP.
//
//	go run ./cmd/linkproxy -listen 127.0.0.1:18102 -target 127.0.0.1:18101 -down 8e6
package main

import (
	"flag"
	"log"
	"net"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18102", "address to listen on")
	target := flag.String("target", "127.0.0.1:18101", "address to forward to")
	down := flag.Float64("down", 8e6, "bits/s to the client (0: no limit)")
	up := flag.Float64("up", 2e6, "bits/s from the client (0: no limit)")
	delay := flag.Duration("delay", 20*time.Millisecond, "one-way delay")
	flag.Parse()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	dl, ul := &link{bps: *down}, &link{bps: *up}
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			s, err := net.Dial("tcp", *target)
			if err != nil {
				c.Close()
				return
			}
			go pipe(s, c, ul, *delay)
			pipe(c, s, dl, *delay)
		}()
	}
}

type chunk struct {
	b  []byte
	at time.Time
}

// pipe copies src to dst through the link, each chunk delayed after it
// passed the bottleneck.
func pipe(dst, src net.Conn, l *link, delay time.Duration) {
	q := make(chan chunk, 64)
	go func() {
		defer dst.Close()
		for c := range q {
			time.Sleep(time.Until(c.at.Add(delay)))
			if _, err := dst.Write(c.b); err != nil {
				return
			}
		}
	}()
	defer close(q)
	for {
		b := make([]byte, 4096)
		n, err := src.Read(b)
		if n > 0 {
			end := l.reserve(n, time.Now())
			time.Sleep(time.Until(end))
			q <- chunk{b[:n], end}
		}
		if err != nil {
			return
		}
	}
}
