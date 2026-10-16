// voicecheck joins a channel as a Mumble client and checks the voice
// packets of everyone who speaks there: frame numbers without gaps or
// repeats, and one terminator per talk spurt. For testing clients
// (browsers) by hand: speak a few spurts, then press Ctrl-C.
//
//	go run ./cmd/voicecheck -server chat.example.org:64738 -channel General
package main

import (
	"crypto/tls"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func main() {
	server := flag.String("server", "localhost:64738", "Mumble server (host:port)")
	name := flag.String("name", "voicecheck", "user name")
	channel := flag.String("channel", "", "channel to join (empty: stay in root)")
	password := flag.String("password", "", "server password")
	insecure := flag.Bool("insecure", false, "accept any server certificate")
	duration := flag.Duration("for", 0, "stop after this long (0: at Ctrl-C)")
	minSpurts := flag.Int("spurts", 1, "fail with fewer finished spurts in all")
	flag.Parse()

	conn, err := tls.Dial("tcp", *server, &tls.Config{InsecureSkipVerify: *insecure})
	if err != nil {
		log.Fatal(err)
	}
	var wmu sync.Mutex
	send := func(typ uint16, m proto.Message) {
		wmu.Lock()
		defer wmu.Unlock()
		if _, err := conn.Write(mumble.MessageFrame(typ, m)); err != nil {
			log.Fatal(err)
		}
	}
	// 1.4: the server sends voice in the legacy format.
	send(mumble.TypeVersion, &pb.Version{VersionV1: proto.Uint32(1<<16 | 4<<8), Release: proto.String("voicecheck")})
	send(mumble.TypeAuthenticate, &pb.Authenticate{Username: name, Password: password, Opus: proto.Bool(true)})
	go func() {
		for range time.Tick(10 * time.Second) {
			send(mumble.TypePing, &pb.Ping{Timestamp: proto.Uint64(uint64(time.Now().Unix()))})
		}
	}()

	var mu sync.Mutex
	names := map[uint32]string{}
	heard := map[uint32][]packet{}
	channels := map[string]uint32{}
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		var timeout <-chan time.Time
		if *duration > 0 {
			timeout = time.After(*duration)
		}
		select {
		case <-stop:
		case <-timeout:
		}
		mu.Lock()
		defer mu.Unlock()
		os.Exit(printReport(names, heard, time.Now(), *minSpurts))
	}()

	header := make([]byte, 6)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			log.Fatal(err)
		}
		typ, body := binary.BigEndian.Uint16(header), make([]byte, binary.BigEndian.Uint32(header[2:]))
		if _, err := io.ReadFull(conn, body); err != nil {
			log.Fatal(err)
		}
		mu.Lock()
		switch typ {
		case mumble.TypeChannelState:
			var m pb.ChannelState
			if proto.Unmarshal(body, &m) == nil && m.Name != nil {
				channels[m.GetName()] = m.GetChannelId()
			}
		case mumble.TypeUserState:
			var m pb.UserState
			if proto.Unmarshal(body, &m) == nil && m.Name != nil {
				names[m.GetSession()] = m.GetName()
			}
		case mumble.TypeReject:
			var m pb.Reject
			proto.Unmarshal(body, &m)
			log.Fatalf("rejected: %s", m.GetReason())
		case mumble.TypeServerSync:
			var m pb.ServerSync
			proto.Unmarshal(body, &m)
			if *channel != "" {
				id, ok := channels[*channel]
				if !ok {
					log.Fatalf("no channel %q", *channel)
				}
				go send(mumble.TypeUserState, &pb.UserState{Session: m.Session, ChannelId: proto.Uint32(id)})
			}
			fmt.Println("listening; speak, then press Ctrl-C")
		case mumble.TypeUDPTunnel:
			if session, p, ok := parseVoice(body); ok {
				p.at = time.Now()
				heard[session] = append(heard[session], p)
				if p.terminator {
					fmt.Printf("%s: end of a spurt at frame %d\n", names[session], p.frame)
				}
			}
		}
		mu.Unlock()
	}
}

// printReport prints each speaker's spurts; the exit code is 1 if any is
// wrong or fewer than minSpurts were finished.
func printReport(names map[uint32]string, heard map[uint32][]packet, end time.Time, minSpurts int) int {
	code, finished := 0, 0
	sessions := slices.Sorted(maps.Keys(heard))
	for _, s := range sessions {
		r := check(heard[s], end)
		status := "OK"
		if !r.ok() {
			status, code = "PROBLEMS", 1
		}
		fmt.Printf("\n%s: %d spurts, %s\n", names[s], len(r.spurts), status)
		for _, sp := range r.spurts {
			fmt.Println("  " + sp.String())
			if !sp.open {
				finished++
			}
		}
	}
	if finished < minSpurts {
		fmt.Printf("\n%d finished spurts, want at least %d\n", finished, minSpurts)
		code = 1
	}
	if len(sessions) == 0 {
		fmt.Println("\nno voice heard")
		code = 1
	}
	return code
}

// parseVoice reads a legacy Opus voice packet from the server: type and
// target, session, frame number, then the Opus header (length and
// terminator bit) and data.
func parseVoice(b []byte) (session uint32, p packet, ok bool) {
	if len(b) < 1 || b[0]>>5 != 4 {
		return 0, p, false
	}
	b = b[1:]
	s, n := legacyVarint(b)
	if n == 0 {
		return 0, p, false
	}
	frame, m := legacyVarint(b[n:])
	if m == 0 {
		return 0, p, false
	}
	h, k := legacyVarint(b[n+m:])
	if k == 0 {
		return 0, p, false
	}
	return uint32(s), packet{frame: frame, terminator: h&0x2000 != 0}, true
}

// legacyVarint reads Mumble's non-negative varint (the length is in the
// first byte's prefix bits); n is 0 on bad input.
func legacyVarint(b []byte) (v uint64, n int) {
	if len(b) == 0 {
		return 0, 0
	}
	c := b[0]
	size, mask := 0, byte(0)
	switch {
	case c&0x80 == 0:
		return uint64(c), 1
	case c&0xc0 == 0x80:
		size, mask = 2, 0x3f
	case c&0xe0 == 0xc0:
		size, mask = 3, 0x1f
	case c&0xf0 == 0xe0:
		size, mask = 4, 0x0f
	case c&0xfc == 0xf0:
		size = 5
	case c&0xfc == 0xf4:
		size = 9
	default:
		return 0, 0
	}
	if len(b) < size {
		return 0, 0
	}
	v = uint64(c & mask)
	for _, x := range b[1:size] {
		v = v<<8 | uint64(x)
	}
	return v, size
}
