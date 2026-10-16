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
	MaxUsers int    // default 100
	MaxPerIP int    // connections per IP (IPv6: per /64); 0: no limit
	// ReservedName reports names (as names.Key) that belong to Kallisti
	// accounts: nobody else may use them. Nil: none.
	ReservedName func(key string) bool
	// WebAccountExists: the Kallisti account with this name and certificate
	// still exists (a guest account can be deleted while it connects).
	// Nil: yes.
	WebAccountExists func(name, certHash string) bool
	// OnChannelText gets every text a client sends to a channel (as plain
	// text), for example to store it. It runs outside the server's lock.
	OnChannelText func(ChannelText)
	// OnRolesChanged runs after admins or moderators changed (see Roles).
	OnRolesChanged func()
	// OnListenOnlyChanged runs after a channel's ACL changed (see
	// ListenOnly).
	OnListenOnlyChanged func()
	// OnUserMoved hears users join (from NoChannel), change channels and
	// leave (to NoChannel), in order, outside the server lock.
	OnUserMoved func(session, from, to uint32)
	DB          *DB // nil: nothing is saved
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

// ChannelText is a text a client sent to a channel (see
// Config.OnChannelText).
type ChannelText struct {
	From       uint32 // session
	Name       string
	Registered bool
	UserID     uint32 // with Registered
	Channel    uint32
	Text       string
	Images     []Image
}

type Server struct {
	cfg Config
	// Taken before mu by changes that are saved, so saves keep their order.
	saveMu sync.Mutex
	// Web account registrations and their removal take turns (see
	// Unregister).
	regMu sync.Mutex

	logins logins
	conns  conns
	pings  *rate.Limiter

	sessions sync.WaitGroup // running connections, for Close
	closing  atomic.Bool
	// Hooks that must not run under mu go through this queue, in order.
	events     chan func()
	stopEvents chan struct{}
	stopOnce   sync.Once
	textParent uint32 // the text channel mirrors' parent (see SyncTextChannels), under mu

	// Read at every accept: not under mu, which the voice path uses.
	bans atomic.Pointer[[]BanEntry]

	mu      sync.Mutex
	state   *State
	clients map[uint32]*client
	byHost  map[netip.Addr]map[*client]bool // for UDP from new addresses
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
	legacy    bool // a client before 1.5: legacy UDP format
	web       bool // a web client through the bridge (ServeConn), not a Mumble client
	limiter   *rate.Limiter
	bandwidth *rate.Limiter // voice bytes per second

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
		byHost:  map[netip.Addr]map[*client]bool{},
	}
	srv.bans.Store(&bans)
	srv.pings = rate.NewLimiter(pingsPerSecond, pingsPerSecond)
	if cfg.OnUserMoved != nil {
		srv.startEvents()
		state.OnMove = func(session, from, to uint32) {
			srv.queue(func() { cfg.OnUserMoved(session, from, to) })
		}
	}
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
		host := remoteHost(conn)
		if s.banned(host, "") != nil || !s.conns.add(host, s.cfg.MaxPerIP) {
			conn.Close()
			continue
		}
		if !s.track() {
			s.conns.remove(host, true)
			conn.Close()
			continue
		}
		go func() {
			defer s.sessions.Done()
			s.handle(conn, Origin{Host: host}, true)
		}()
	}
}

// track counts a new connection, unless the server is closing.
func (s *Server) track() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing.Load() {
		return false
	}
	s.sessions.Add(1)
	return true
}

// Close disconnects all clients and waits (briefly) until their sessions
// ended, so last channels are saved before the database closes.
func (s *Server) Close() {
	s.mu.Lock()
	s.closing.Store(true)
	if s.stopEvents != nil {
		s.stopOnce.Do(func() { close(s.stopEvents) })
	}
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()
	for _, c := range clients {
		c.close()
	}
	done := make(chan struct{})
	go func() {
		s.sessions.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
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
