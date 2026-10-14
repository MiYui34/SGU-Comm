// Package mumbled is the built-in Mumble server. The state (channels,
// users) is in State, without IO; this file has the connections, voice and
// UDP, messages.go the control messages. Only clients from 1.5 on (the
// protobuf UDP format) are accepted for now.
package mumbled

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/cryptstate"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

type Config struct {
	Welcome  string
	RootName string // default "Root"
	Password string // for guests; registered users need none
	DB       *DB    // nil: nothing is saved
}

const (
	handshakeTimeout = 10 * time.Second
	// Mumble clients ping every 5 s; Murmur drops them after 30 s.
	idleTimeout  = 30 * time.Second
	writeTimeout = 10 * time.Second
	// When a connection ends, queued frames (a kick reason) still go out.
	flushTimeout = 2 * time.Second
	// Before Authenticate frames stay small; after it, nothing in the
	// spike needs more (no avatars yet).
	maxHandshakeFrame = 64 << 10
	maxFrame          = 128 << 10
	queueSize         = 512
	maxBandwidth      = 558000
	maxTextLength     = 5000
	// The CELT version Murmur announces (0.7.0) next to Opus.
	celt070 = -2147483637
	// Murmur's default messagelimit and messageburst: text messages and
	// state changes per second.
	messageLimit = 1
	messageBurst = 5
)

var (
	versionV1 = uint32(1<<16 | 5<<8)
	versionV2 = uint64(1<<48 | 5<<32)
	// As Murmur: ask the client for a new nonce when its UDP packets did
	// not decrypt for this long, and not more often. A variable for tests.
	resyncAfter = 5 * time.Second
)

type Server struct {
	cfg Config
	// Taken before mu by changes that are saved, so saves keep their order.
	saveMu sync.Mutex

	logins logins

	// Read at every accept: not under mu, which the voice path uses.
	bans atomic.Pointer[[]BanEntry]

	mu      sync.Mutex
	state   *State
	clients map[uint32]*client
	byAddr  map[netip.AddrPort]*client
	next    uint32
	udp     *net.UDPConn
}

type client struct {
	conn      net.Conn
	closeOnce sync.Once
	out       chan []byte
	session   uint32
	name      string
	host      netip.Addr
	limiter   *rate.Limiter

	// Guarded by Server.mu.
	udpAddr  netip.AddrPort
	udpVoice bool // send voice over UDP (it sent voice over UDP)

	cryptMu     sync.Mutex
	crypt       *cryptstate.CryptState
	lastGood    time.Time
	lastRequest time.Time
}

func New(cfg Config) (*Server, error) {
	if cfg.RootName == "" {
		cfg.RootName = "Root"
	}
	var saved []Channel
	var lastID uint32
	if cfg.DB != nil {
		var err error
		if saved, err = cfg.DB.Channels(); err != nil {
			return nil, err
		}
		if lastID, err = cfg.DB.NextChannel(); err != nil {
			return nil, err
		}
	}
	var bans []BanEntry
	if cfg.DB != nil {
		var err error
		if bans, err = cfg.DB.Bans(); err != nil {
			return nil, err
		}
	}
	state := NewState(cfg.RootName, saved)
	state.SetLastChannelID(lastID)
	if cfg.DB == nil {
		state.Channels[0].ACLs, state.Channels[0].Groups = DefaultRootACL()
	}
	srv := &Server{
		cfg:     cfg,
		state:   state,
		clients: map[uint32]*client{},
		byAddr:  map[netip.AddrPort]*client{},
	}
	srv.bans.Store(&bans)
	return srv, nil
}

// Serve accepts TLS connections until the listener closes.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		// Banned addresses are dropped before TLS, as in Murmur.
		if s.banned(remoteHost(conn), "") != nil {
			conn.Close()
			continue
		}
		go s.handle(conn)
	}
}

// Close disconnects all clients.
func (s *Server) Close() {
	s.mu.Lock()
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()
	for _, c := range clients {
		c.close()
	}
}

func readFrame(conn net.Conn, limit int) (uint16, []byte, error) {
	var header [6]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(header[2:])
	if n > uint32(limit) {
		return 0, nil, errors.New("frame too large")
	}
	body := make([]byte, n)
	_, err := io.ReadFull(conn, body)
	return binary.BigEndian.Uint16(header[:]), body, err
}

// close never blocks: closing a TLS connection waits for a writer that
// may be stuck, and callers hold locks.
func (c *client) close() {
	c.closeOnce.Do(func() { go c.conn.Close() })
}

// send queues a frame. A full queue drops voice; for anything else the
// client is too slow and is disconnected.
func (c *client) send(frame []byte, voice bool) {
	select {
	case c.out <- frame:
	default:
		if !voice {
			c.close()
		}
	}
}

// closeAfterFlush closes the connection once the queued frames are sent.
func (c *client) closeAfterFlush() {
	select {
	case c.out <- nil:
	default:
		c.close()
	}
}

func (c *client) sendMsg(typ uint16, m proto.Message) {
	c.send(mumble.MessageFrame(typ, m), false)
}

// writer sends the queue until closeAfterFlush or a write error.
func (c *client) writer() {
	for b := range c.out {
		if b == nil { // closeAfterFlush
			c.close()
			return
		}
		c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := c.conn.Write(b); err != nil {
			c.close()
			return
		}
	}
}

func (c *client) stats() *pb.UserStats_Stats {
	c.cryptMu.Lock()
	defer c.cryptMu.Unlock()
	return &pb.UserStats_Stats{
		Good: proto.Uint32(c.crypt.Good), Late: proto.Uint32(c.crypt.Late), Lost: proto.Uint32(c.crypt.Lost),
	}
}

// banned returns the ban that applies to an address or certificate.
func (s *Server) banned(ip netip.Addr, hash string) *BanEntry {
	now := time.Now()
	for _, b := range s.currentBans() {
		if b.Matches(ip, hash, now) {
			return &b
		}
	}
	return nil
}

// remoteHost is the client's address as bans and limits see it: IPv4 as
// IPv4, without an IPv6 zone (a zone never matches a prefix).
func remoteHost(conn net.Conn) netip.Addr {
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return tcp.AddrPort().Addr().Unmap().WithZone("")
	}
	return netip.Addr{}
}
