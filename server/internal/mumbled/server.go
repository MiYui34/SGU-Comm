// Package mumbled is the built-in Mumble server. The state (channels,
// users) is in State, without IO; this file has the connections, voice and
// UDP, messages.go the control messages. Only clients from 1.5 on (the
// protobuf UDP format) are accepted for now.
package mumbled

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/cryptstate"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/names"
)

type Config struct {
	Welcome  string
	RootName string // default "Root"
	DB       *DB    // nil: nothing is saved
}

const (
	handshakeTimeout = 10 * time.Second
	// Mumble clients ping every 5 s; Murmur drops them after 30 s.
	idleTimeout  = 30 * time.Second
	writeTimeout = 10 * time.Second
	// Before Authenticate frames stay small; after it, nothing in the
	// spike needs more (no avatars yet).
	maxHandshakeFrame = 64 << 10
	maxFrame          = 128 << 10
	queueSize         = 512
	maxBandwidth      = 558000
	maxTextLength     = 5000
	// Traverse, Enter, Speak, TextMessage.
	permissions = 0x2 | 0x4 | 0x8 | 0x200
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
	state := NewState(cfg.RootName, saved)
	state.SetLastChannelID(lastID)
	return &Server{
		cfg:     cfg,
		state:   state,
		clients: map[uint32]*client{},
		byAddr:  map[netip.AddrPort]*client{},
	}, nil
}

// Serve accepts TLS connections until the listener closes.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
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

func (c *client) sendMsg(typ uint16, m proto.Message) {
	c.send(mumble.MessageFrame(typ, m), false)
}

func (c *client) writer(done <-chan struct{}) {
	for {
		select {
		case b := <-c.out:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := c.conn.Write(b); err != nil {
				c.close()
				return
			}
		case <-done:
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

func clientVersion(v *pb.Version) uint64 {
	if v.VersionV2 != nil {
		return v.GetVersionV2()
	}
	v1 := uint64(v.GetVersionV1())
	return (v1>>16)<<48 | (v1>>8&0xff)<<32 | (v1&0xff)<<16
}

func random16() (b [16]byte) {
	rand.Read(b[:])
	return b
}

// handshake reads Version and Authenticate. It returns a Reject frame if
// the client cannot join.
func handshake(conn net.Conn) (name string, reject []byte, err error) {
	var version pb.Version
	for {
		typ, body, err := readFrame(conn, maxHandshakeFrame)
		if err != nil {
			return "", nil, err
		}
		switch typ {
		case mumble.TypeVersion:
			proto.Unmarshal(body, &version)
		case mumble.TypeAuthenticate:
			var auth pb.Authenticate
			proto.Unmarshal(body, &auth)
			if clientVersion(&version) < versionV2 {
				return "", mumble.RejectFrame(pb.Reject_WrongVersion, "This server needs Mumble 1.5 or newer."), nil
			}
			name = strings.TrimSpace(auth.GetUsername())
			if why := names.Check(name); why != "" {
				return "", mumble.RejectFrame(pb.Reject_InvalidUsername, why), nil
			}
			return name, nil, nil
		}
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	name, reject, err := handshake(conn)
	if err != nil {
		return
	}
	if reject != nil {
		conn.Write(reject)
		return
	}
	key, clientNonce, serverNonce := random16(), random16(), random16()
	c := &client{
		conn:     conn,
		out:      make(chan []byte, queueSize),
		name:     name,
		limiter:  rate.NewLimiter(messageLimit, messageBurst),
		crypt:    cryptstate.New(key, serverNonce, clientNonce),
		lastGood: time.Now(),
	}
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		c.host = tcp.AddrPort().Addr().Unmap()
	}
	setup := &pb.CryptSetup{Key: key[:], ClientNonce: clientNonce[:], ServerNonce: serverNonce[:]}
	if !s.join(c, setup) {
		conn.Write(mumble.RejectFrame(pb.Reject_UsernameInUse, "This name is in use."))
		return
	}
	defer s.leave(c)
	done := make(chan struct{})
	defer close(done)
	go c.writer(done)
	conn.SetDeadline(time.Time{})
	for {
		conn.SetReadDeadline(time.Now().Add(idleTimeout))
		typ, body, err := readFrame(conn, maxFrame)
		if err != nil {
			return
		}
		s.message(c, typ, body)
	}
}

// join adds the client if its name is free. The whole initial state goes
// into its queue under the lock, so no later change can overtake it.
func (s *Server) join(c *client, setup *pb.CryptSetup) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := names.Key(c.name)
	for _, u := range s.state.Users {
		if names.Key(u.Name) == key {
			return false
		}
	}
	s.next++
	c.session = s.next
	c.sendMsg(mumble.TypeVersion, &pb.Version{
		VersionV1: proto.Uint32(versionV1), VersionV2: proto.Uint64(versionV2), Release: proto.String("Kallisti"),
	})
	c.sendMsg(mumble.TypeCryptSetup, setup)
	c.sendMsg(mumble.TypeCodecVersion, &pb.CodecVersion{
		Alpha: proto.Int32(celt070), Beta: proto.Int32(0), PreferAlpha: proto.Bool(true), Opus: proto.Bool(true),
	})
	tree := s.state.Tree()
	for _, ch := range tree {
		c.sendMsg(mumble.TypeChannelState, channelState(ch))
	}
	// Links once all channels exist, as Murmur does.
	for _, ch := range tree {
		if len(ch.Links) > 0 {
			c.sendMsg(mumble.TypeChannelState, &pb.ChannelState{ChannelId: proto.Uint32(ch.ID), Links: slices.Sorted(maps.Keys(ch.Links))})
		}
	}
	for _, u := range s.state.Users {
		c.send(userState(u), false)
	}
	u := &User{Session: c.session, Name: c.name}
	s.state.AddUser(u)
	s.clients[c.session] = c
	s.broadcastLocked(userState(u), nil)
	c.sendMsg(mumble.TypeServerSync, &pb.ServerSync{
		Session: proto.Uint32(c.session), MaxBandwidth: proto.Uint32(maxBandwidth),
		WelcomeText: proto.String(s.cfg.Welcome), Permissions: proto.Uint64(permissions),
	})
	c.sendMsg(mumble.TypeServerConfig, &pb.ServerConfig{
		AllowHtml: proto.Bool(false), MessageLength: proto.Uint32(maxTextLength),
	})
	return true
}

func (s *Server) leave(c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c.session)
	if s.byAddr[c.udpAddr] == c {
		delete(s.byAddr, c.udpAddr)
	}
	removed := s.state.RemoveUser(c.session)
	s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(c.session)}), nil)
	s.broadcastRemovedLocked(removed)
}

// channelState is the full ChannelState of a channel, without links.
func channelState(ch *Channel) *pb.ChannelState {
	m := &pb.ChannelState{
		ChannelId: proto.Uint32(ch.ID), Name: proto.String(ch.Name), Description: proto.String(ch.Description),
		Position: proto.Int32(ch.Position), Temporary: proto.Bool(ch.Temporary),
	}
	if ch.ID != 0 {
		m.Parent = proto.Uint32(ch.Parent)
	}
	return m
}

// userState is the full UserState of a user.
func userState(u *User) []byte {
	return mumble.MessageFrame(mumble.TypeUserState, &pb.UserState{
		Session: proto.Uint32(u.Session), Name: proto.String(u.Name), ChannelId: proto.Uint32(u.Channel),
		SelfMute: proto.Bool(u.SelfMute), SelfDeaf: proto.Bool(u.SelfDeaf),
		Mute: proto.Bool(u.Mute), Deaf: proto.Bool(u.Deaf), Suppress: proto.Bool(u.Suppress),
	})
}

// broadcastLocked sends a frame to everyone except skip. Needs Server.mu.
func (s *Server) broadcastLocked(frame []byte, skip *client) {
	for _, c := range s.clients {
		if c != skip {
			c.send(frame, false)
		}
	}
}

// broadcastRemovedLocked tells everyone about removed channels.
func (s *Server) broadcastRemovedLocked(removed []uint32) {
	for _, id := range removed {
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeChannelRemove, &pb.ChannelRemove{ChannelId: proto.Uint32(id)}), nil)
	}
}

// relayed is the audio packet as receivers get it: the server sets the
// context (normal talking) and the sender's session. ok is false for
// packets that are not audio to the channel.
func relayed(packet []byte, sender uint32) (out []byte, ok bool) {
	if len(packet) == 0 || packet[0] != 0 {
		return nil, false
	}
	out = []byte{0}
	out = protowire.AppendTag(out, 2, protowire.VarintType)
	out = protowire.AppendVarint(out, 0)
	out = protowire.AppendTag(out, 3, protowire.VarintType)
	out = protowire.AppendVarint(out, uint64(sender))
	b := packet[1:]
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, false
		}
		m := protowire.ConsumeFieldValue(num, typ, b[n:])
		if m < 0 {
			return nil, false
		}
		switch num {
		case 1: // target: whispers are not part of the spike
			if v, _ := protowire.ConsumeVarint(b[n:]); typ != protowire.VarintType || v != 0 {
				return nil, false
			}
		case 2, 3: // set by the server
		default:
			out = append(out, b[:n+m]...)
		}
		b = b[n+m:]
	}
	return out, true
}

// voice relays an audio packet from c to everyone else who listens.
func (s *Server) voice(c *client, packet []byte) {
	out, ok := relayed(packet, c.session)
	if !ok {
		return
	}
	type target struct {
		c    *client
		addr netip.AddrPort
	}
	s.mu.Lock()
	var targets []target
	for _, session := range s.state.Listeners(c.session) {
		o := s.clients[session]
		t := target{c: o}
		if o.udpVoice && s.udp != nil {
			t.addr = o.udpAddr
		}
		targets = append(targets, t)
	}
	udp := s.udp
	s.mu.Unlock()
	tunnel := mumble.EncodeFrame(mumble.TypeUDPTunnel, out)
	for _, t := range targets {
		if t.addr.IsValid() {
			t.c.cryptMu.Lock()
			packet := t.c.crypt.Encrypt(out)
			t.c.cryptMu.Unlock()
			udp.WriteToUDPAddrPort(packet, t.addr)
		} else {
			t.c.send(tunnel, true)
		}
	}
}

// ServeUDP handles UDP voice and pings until the socket closes.
func (s *Server) ServeUDP(conn *net.UDPConn) error {
	s.mu.Lock()
	s.udp = conn
	s.mu.Unlock()
	buf := make([]byte, 2048)
	for {
		n, addr, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			return err
		}
		s.udpPacket(conn, addr, buf[:n])
	}
}

// decrypt tries a client's key. If its packets stopped decrypting, it
// returns true for requestSync (as Murmur's checkDecrypt).
func (c *client) decrypt(packet []byte, now time.Time) (plain []byte, ok, requestSync bool) {
	c.cryptMu.Lock()
	defer c.cryptMu.Unlock()
	if plain, ok = c.crypt.Decrypt(packet); ok {
		c.lastGood = now
		return plain, true, false
	}
	if now.Sub(c.lastGood) > resyncAfter && now.Sub(c.lastRequest) > resyncAfter {
		c.lastRequest = now
		return nil, false, true
	}
	return nil, false, false
}

func (s *Server) udpPacket(conn *net.UDPConn, addr netip.AddrPort, packet []byte) {
	addr = netip.AddrPortFrom(addr.Addr().Unmap(), addr.Port())
	// A known address, or else: try the clients from the same IP, as
	// Murmur does.
	s.mu.Lock()
	known := s.byAddr[addr]
	candidates := []*client{known}
	if known == nil {
		candidates = candidates[:0]
		for _, c := range s.clients {
			if c.host == addr.Addr() {
				candidates = append(candidates, c)
			}
		}
	}
	s.mu.Unlock()
	now := time.Now()
	for _, c := range candidates {
		plain, ok, requestSync := c.decrypt(packet, now)
		if requestSync && c == known {
			c.sendMsg(mumble.TypeCryptSetup, &pb.CryptSetup{})
		}
		if !ok {
			continue
		}
		s.mu.Lock()
		if c.udpAddr != addr {
			if s.byAddr[c.udpAddr] == c {
				delete(s.byAddr, c.udpAddr)
			}
			c.udpAddr = addr
			s.byAddr[addr] = c
		}
		// As Murmur: only voice (not pings) over UDP means the client
		// gets voice over UDP.
		isAudio := len(plain) > 0 && plain[0] == 0
		if isAudio {
			c.udpVoice = true
		}
		s.mu.Unlock()
		if isAudio {
			s.voice(c, plain)
		} else if len(plain) > 0 && plain[0] == 1 {
			// Pings come back as they are.
			c.cryptMu.Lock()
			pong := c.crypt.Encrypt(plain)
			c.cryptMu.Unlock()
			conn.WriteToUDPAddrPort(pong, addr)
		}
		return
	}
}
