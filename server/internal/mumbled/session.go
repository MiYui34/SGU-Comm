package mumbled

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
	"kallisti.chat/server/internal/cryptstate"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/names"
	"kallisti.chat/server/internal/password"
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
type hello struct {
	name, password string
	tokens         []string
	legacy         bool // before 1.5: legacy UDP format
	noOpus         bool
}

func handshake(conn net.Conn) (h hello, reject []byte, err error) {
	var version pb.Version
	for {
		typ, body, err := readFrame(conn, maxHandshakeFrame)
		if err != nil {
			return h, nil, err
		}
		switch typ {
		case mumble.TypeVersion:
			proto.Unmarshal(body, &version)
		case mumble.TypeAuthenticate:
			var auth pb.Authenticate
			proto.Unmarshal(body, &auth)
			h = hello{name: strings.TrimSpace(auth.GetUsername()), password: auth.GetPassword(), tokens: auth.Tokens,
				legacy: clientVersion(&version) < versionV2, noOpus: !auth.GetOpus()}
			return h, nil, nil
		}
	}
}

// TLSConfig asks clients for their certificate (any certificate: Mumble
// clients have self-signed ones), which identifies registered users.
func TLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequestClientCert, MinVersion: tls.VersionTLS12}
}

// certHash is the SHA-1 of the client certificate, as Murmur shows it.
func certHash(conn net.Conn) string {
	tc, ok := conn.(*tls.Conn)
	if !ok {
		return ""
	}
	peer := tc.ConnectionState().PeerCertificates
	if len(peer) == 0 {
		return ""
	}
	sum := sha1.Sum(peer[0].Raw)
	return hex.EncodeToString(sum[:])
}

// identity is who a connection is, after authentication.
type identity struct {
	name        string
	registered  bool // the zero value is a guest
	userID      uint32
	hash        string
	tokens      []string
	lastChannel uint32
	comment     string
	texture     []byte
}

func (s *Server) identify(h hello, hash string, host netip.Addr) (identity, []byte) {
	if s.logins.blocked(host) {
		return identity{}, mumble.RejectFrame(pb.Reject_None, "Too many failed logins. Please try again later.")
	}
	if b := s.banned(host, hash); b != nil {
		reason := "You are banned from this server."
		if b.Reason != "" {
			reason = "You are banned from this server: " + b.Reason
		}
		return identity{}, mumble.RejectFrame(pb.Reject_None, reason)
	}
	var byName, byHash *Registered
	if db := s.cfg.DB; db != nil {
		var err1, err2 error
		byName, err1 = db.UserByName(h.name)
		byHash, err2 = db.UserByHash(hash)
		if err := errors.Join(err1, err2); err != nil {
			slog.Error("looking up a user", "err", err)
			return identity{}, mumble.RejectFrame(pb.Reject_AuthenticatorFail, "Please try again later.")
		}
	}
	r, ok := authenticate(byName, byHash, h.password, hash, checkPassword)
	if !ok {
		s.logins.failed(host)
		return identity{}, mumble.RejectFrame(pb.Reject_WrongUserPW, "Wrong certificate or password for existing user.")
	}
	if r != nil {
		comment, texture, err := s.cfg.DB.UserContent(r.ID)
		if err != nil {
			slog.Error("loading a comment", "user", r.ID, "err", err)
		}
		return identity{name: r.Name, registered: true, userID: r.ID, hash: hash, tokens: h.tokens,
			lastChannel: r.LastChannel, comment: comment, texture: texture}, nil
	}
	if why := names.Check(h.name); why != "" {
		return identity{}, mumble.RejectFrame(pb.Reject_InvalidUsername, why)
	}
	if s.cfg.Password != "" && subtle.ConstantTimeCompare([]byte(h.password), []byte(s.cfg.Password)) != 1 {
		s.logins.failed(host)
		return identity{}, mumble.RejectFrame(pb.Reject_WrongServerPW, "Wrong server password.")
	}
	return identity{name: h.name, hash: hash, tokens: h.tokens}, nil
}

// checkPassword runs argon2 with the limit on parallel hashes.
func checkPassword(encoded, pw string) bool {
	defer password.Limit()()
	return password.Check(encoded, pw)
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	h, reject, err := handshake(conn)
	if err != nil {
		return
	}
	host := remoteHost(conn)
	id := identity{}
	if reject == nil {
		id, reject = s.identify(h, certHash(conn), host)
	}
	if reject != nil {
		conn.Write(reject)
		return
	}
	key, clientNonce, serverNonce := random16(), random16(), random16()
	c := &client{
		conn:      conn,
		out:       make(chan []byte, queueSize),
		name:      id.name,
		limiter:   rate.NewLimiter(messageLimit, messageBurst),
		bandwidth: rate.NewLimiter(maxBandwidth/8, maxBandwidth/8),
		crypt:     cryptstate.New(key, serverNonce, clientNonce),
		lastGood:  time.Now(),
		host:      host,
		legacy:    h.legacy,
	}
	setup := &pb.CryptSetup{Key: key[:], ClientNonce: clientNonce[:], ServerNonce: serverNonce[:]}
	if reject := s.join(c, id, setup); reject != nil {
		conn.Write(reject)
		return
	}
	if h.noOpus { // as Murmur: such clients can neither talk nor hear
		c.sendMsg(mumble.TypeTextMessage, &pb.TextMessage{Message: proto.String(
			"Your client does not support the Opus codec: you will not be able to talk or hear. Please update Mumble.")})
	}
	written := make(chan struct{})
	go func() {
		c.writer()
		close(written)
	}()
	defer func() {
		c.closeAfterFlush()
		select {
		case <-written:
		case <-time.After(flushTimeout):
		}
	}()
	defer s.leave(c) // runs first: others see the user go at once
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
// join returns a Reject frame if the client cannot join.
func (s *Server) join(c *client, id identity, setup *pb.CryptSetup) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	online := make([]Presence, 0, len(s.state.Users))
	for _, u := range s.state.Users {
		online = append(online, Presence{
			Session: u.Session, Name: u.Name, Registered: u.Registered, UserID: u.UserID,
			Host: s.clients[u.Session].host, Hash: u.Hash,
		})
	}
	ghost, inUse := findGhost(online, Presence{Name: id.name, Registered: id.registered, UserID: id.userID, Host: c.host, Hash: id.hash})
	if inUse {
		return mumble.RejectFrame(pb.Reject_UsernameInUse, "This name is in use.")
	}
	if g := s.state.Users[ghost]; g != nil {
		if g.Registered && id.registered && g.UserID == id.userID {
			id.lastChannel = g.Channel // the same user on another device stays where it was
		}
		s.kickLocked(ghost, 0, "You connected to the server from another device.", false)
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
	u := &User{Session: c.session, Name: c.name, Registered: id.registered, UserID: id.userID, Hash: id.hash, Tokens: id.tokens,
		Comment: id.comment, Texture: id.texture}
	// The last channel if the user may enter it, else root (as Murmur).
	if ch := s.state.Channels[id.lastChannel]; ch != nil && s.state.Has(u, ch.ID, Enter) {
		u.Channel = ch.ID
	}
	if !s.state.Has(u, u.Channel, Enter) {
		return mumble.RejectFrame(pb.Reject_None, "You may not enter this server.")
	}
	s.state.Refresh(u)
	tree := s.state.Tree()
	for _, ch := range tree {
		c.sendMsg(mumble.TypeChannelState, s.state.channelStateFor(ch, u))
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
	s.state.AddUser(u)
	s.clients[c.session] = c
	s.broadcastLocked(userState(u), nil)
	c.sendMsg(mumble.TypeServerSync, &pb.ServerSync{
		Session: proto.Uint32(c.session), MaxBandwidth: proto.Uint32(maxBandwidth),
		WelcomeText: proto.String(s.cfg.Welcome), Permissions: proto.Uint64(s.state.rootPerms(u)),
	})
	c.sendMsg(mumble.TypeServerConfig, &pb.ServerConfig{
		AllowHtml: proto.Bool(false), MessageLength: proto.Uint32(maxTextLength),
	})
	return nil
}

func (s *Server) leave(c *client) {
	s.mu.Lock()
	if s.clients[c.session] != c {
		s.mu.Unlock()
		return // already replaced or kicked
	}
	u := s.removeLocked(c, &pb.UserRemove{Session: proto.Uint32(c.session)})
	s.mu.Unlock()
	s.saveLastChannel(u)
}

func (s *Server) saveLastChannel(u User) {
	if !u.Registered || s.cfg.DB == nil {
		return
	}
	if err := s.cfg.DB.SetLastChannel(u.UserID, u.Channel); err != nil {
		slog.Error("saving the last channel", "user", u.UserID, "err", err)
	}
}

// removeLocked takes a client off the server, tells everyone, and returns
// the user as it was.
func (s *Server) removeLocked(c *client, msg *pb.UserRemove) User {
	u := *s.state.Users[c.session]
	delete(s.clients, c.session)
	if s.byAddr[c.udpAddr] == c {
		delete(s.byAddr, c.udpAddr)
	}
	removed := s.state.RemoveUser(c.session)
	s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserRemove, msg), nil)
	s.broadcastRemovedLocked(removed)
	return u
}

// kickLocked disconnects a user: it gets the reason, everyone else the
// removal. actor 0: the server.
func (s *Server) kickLocked(session, actor uint32, reason string, ban bool) {
	c := s.clients[session]
	if c == nil {
		return
	}
	msg := &pb.UserRemove{Session: proto.Uint32(session), Reason: proto.String(reason)}
	if ban {
		msg.Ban = proto.Bool(true)
	}
	if actor != 0 {
		msg.Actor = proto.Uint32(actor)
	}
	c.sendMsg(mumble.TypeUserRemove, msg)
	c.closeAfterFlush()
	c.conn.SetReadDeadline(time.Now()) // nothing more from it
	u := s.removeLocked(c, msg)
	go s.saveLastChannel(u)
}

// channelState is the full ChannelState of a channel, without links.
func channelState(ch *Channel) *pb.ChannelState {
	m := &pb.ChannelState{
		ChannelId: proto.Uint32(ch.ID), Name: proto.String(ch.Name),
		Position: proto.Int32(ch.Position), Temporary: proto.Bool(ch.Temporary),
	}
	m.Description, m.DescriptionHash = commentFields(ch.Description)
	if ch.ID != 0 {
		m.Parent = proto.Uint32(ch.Parent)
	}
	return m
}

// userState is the full UserState of a user.
func userState(u *User) []byte {
	m := &pb.UserState{
		Session: proto.Uint32(u.Session), Name: proto.String(u.Name), ChannelId: proto.Uint32(u.Channel),
		SelfMute: proto.Bool(u.SelfMute), SelfDeaf: proto.Bool(u.SelfDeaf),
		Mute: proto.Bool(u.Mute), Deaf: proto.Bool(u.Deaf), Suppress: proto.Bool(u.Suppress),
		PrioritySpeaker: proto.Bool(u.PrioritySpeaker), Recording: proto.Bool(u.Recording),
	}
	if u.Registered {
		m.UserId = proto.Uint32(u.UserID)
	}
	if u.Hash != "" {
		m.Hash = proto.String(u.Hash)
	}
	m.ListeningChannelAdd = slices.Sorted(maps.Keys(u.Listening))
	setUserBlobs(m, u)
	return mumble.MessageFrame(mumble.TypeUserState, m)
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
