// Package mcstatus reads a Minecraft server's player count over the server
// list ping: the exchange the game client itself does to show the MOTD, so
// nothing secret is kept on this side and nothing else has to be opened on
// the game server.
package mcstatus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// The protocol version the handshake claims: -1 asks for no comparison at
// all, so the server answers with whatever version it runs.
const protocolVersion = -1

// Bounds of the answer: the player list of a full server fits well inside.
const maxReply = 1 << 20

// Reply is what the web app shows of the server.
type Reply struct {
	Online  int      `json:"online"`
	Slots   int      `json:"slots"`
	Version string   `json:"version,omitempty"`
	Players []string `json:"players"`
	// RTT is this server's round trip to the game server, in milliseconds:
	// the second half of the delay a player sees, the first half being the
	// browser's own round trip to this server.
	RTT int64 `json:"rtt"`
	// At is when this answer was read (Unix milliseconds).
	At int64 `json:"at"`
}

// Status is one Minecraft server, at most asked once per Cache for every
// caller: a page that polls it must not become a flood of the game server.
type Status struct {
	// Addr is the host:port of the game server, as players reach it.
	Addr    string
	Timeout time.Duration // 0: 3 seconds
	Cache   time.Duration // 0: 5 seconds
	Now     func() time.Time

	mu    sync.Mutex
	reply Reply
	err   error
	at    time.Time
}

func (s *Status) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 3 * time.Second
}

func (s *Status) cache() time.Duration {
	if s.Cache > 0 {
		return s.Cache
	}
	return 5 * time.Second
}

func (s *Status) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Get is the last answer, asking the server when the cached one is older
// than Cache. A failed question is cached too, with the error it returned.
func (s *Status) Get(ctx context.Context) (Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if !s.at.IsZero() && now.Sub(s.at) < s.cache() {
		return s.reply, s.err
	}
	reply, err := s.ask(ctx)
	if err != nil {
		reply = Reply{Players: []string{}, At: now.UnixMilli()}
		slog.Warn("status of the Minecraft server", "addr", s.Addr, "error", err)
	}
	s.at, s.reply, s.err = now, reply, err
	return reply, err
}

// ask does one full exchange: connect, handshake, status request, answer.
func (s *Status) ask(ctx context.Context) (Reply, error) {
	host, portText, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return Reply{}, fmt.Errorf("MC_STATUS_ADDR %q: %w", s.Addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return Reply{}, fmt.Errorf("MC_STATUS_ADDR %q: %q is not a port", s.Addr, portText)
	}
	dialer := &net.Dialer{Timeout: s.timeout()}
	conn, err := dialer.DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return Reply{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(s.now().Add(s.timeout())); err != nil {
		return Reply{}, err
	}
	// The time from the status request to the answer is one round trip: the
	// connection is already open, so this is the network and the server's
	// own reading of the question, nothing else.
	started := time.Now()
	raw, err := ping(conn, host, port)
	if err != nil {
		return Reply{}, err
	}
	rtt := time.Since(started)
	var answer statusAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return Reply{}, fmt.Errorf("answer of %s: %w", s.Addr, err)
	}
	players := make([]string, 0, len(answer.Players.Sample))
	for _, player := range answer.Players.Sample {
		if player.Name != "" {
			players = append(players, player.Name)
		}
	}
	return Reply{
		Online:  answer.Players.Online,
		Slots:   answer.Players.Max,
		Version: answer.Version.Name,
		Players: players,
		RTT:     int64(rtt.Round(time.Millisecond) / time.Millisecond),
		At:      s.now().UnixMilli(),
	}, nil
}

// statusAnswer is the part of the status JSON read here.
type statusAnswer struct {
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
		Sample []struct {
			Name string `json:"name"`
		} `json:"sample"`
	} `json:"players"`
}

// ping does the server list ping: handshake, status request, status answer.
func ping(conn net.Conn, host string, port int) ([]byte, error) {
	var handshake bytes.Buffer
	writeVarint(&handshake, 0x00) // handshake
	writeVarint(&handshake, protocolVersion)
	writeString(&handshake, host)
	var number [2]byte
	binary.BigEndian.PutUint16(number[:], uint16(port))
	handshake.Write(number[:])
	writeVarint(&handshake, 1) // next state: status

	// Both packets go out together: the answer follows the request.
	var out bytes.Buffer
	writePacket(&out, handshake.Bytes())
	writePacket(&out, []byte{0x00}) // status request
	if _, err := conn.Write(out.Bytes()); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	length, err := readVarint(r)
	if err != nil {
		return nil, err
	}
	if length <= 0 || length > maxReply {
		return nil, fmt.Errorf("answer of %d bytes", length)
	}
	if id, err := readVarint(r); err != nil {
		return nil, err
	} else if id != 0x00 {
		return nil, fmt.Errorf("packet %#x: not a status answer", id)
	}
	size, err := readVarint(r)
	if err != nil {
		return nil, err
	}
	if size < 0 || size > maxReply {
		return nil, fmt.Errorf("status text of %d bytes", size)
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// writePacket writes one packet: its length, then its body.
func writePacket(b *bytes.Buffer, body []byte) {
	writeVarint(b, int32(len(body)))
	b.Write(body)
}

// writeVarint writes a protocol varint: seven bits a byte, the high bit set
// on all but the last.
func writeVarint(b *bytes.Buffer, value int32) {
	v := uint32(value)
	for v >= 0x80 {
		b.WriteByte(byte(v) | 0x80)
		v >>= 7
	}
	b.WriteByte(byte(v))
}

func writeString(b *bytes.Buffer, s string) {
	writeVarint(b, int32(len(s)))
	b.WriteString(s)
}

// readVarint reads a protocol varint, of at most five bytes.
func readVarint(r io.ByteReader) (int32, error) {
	var value uint32
	for i := range 5 {
		c, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(c&0x7f) << (7 * uint(i))
		if c&0x80 == 0 {
			return int32(value), nil
		}
	}
	return 0, errors.New("varint of more than five bytes")
}

// ServeHTTP is GET /api/mc/status: the counts, never the address, so that the
// page shows the server state without reaching the game server itself.
func (s *Status) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
		return
	}
	if s.Addr == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "configured": false})
		return
	}
	reply, err := s.Get(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "configured": true, "error": "cannot reach the game server"})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		OK bool `json:"ok"`
		Reply
	}{true, reply})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
}
