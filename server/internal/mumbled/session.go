package mumbled

import (
	"crypto/rand"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
	"kallisti.chat/server/internal/cryptstate"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/names"
)

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
