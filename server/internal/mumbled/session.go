package mumbled

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

// TLSConfigFrom asks get for the certificate on each handshake (for
// certificates that are renewed while the server runs).
func TLSConfigFrom(get func(*tls.ClientHelloInfo) (*tls.Certificate, error)) *tls.Config {
	return &tls.Config{GetCertificate: get, ClientAuth: tls.RequestClientCert, MinVersion: tls.VersionTLS12}
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

// Origin is where a connection comes from, for connections that are not
// accepted by Serve (the Kallisti server's web bridge).
type Origin struct {
	Host    netip.Addr  // the real client address
	Account *WebAccount // a Kallisti account, or nil for a guest
}

// WebAccount is a Kallisti account: a registered user identified by the
// certificate it has (it never connects with TLS itself).
type WebAccount struct {
	Name     string
	CertHash string
}

// ServeConn serves one connection without TLS, for example one end of a
// net.Pipe. Limits per IP are the caller's job.
func (s *Server) ServeConn(conn net.Conn, o Origin) {
	if s.banned(o.Host, "") != nil || !s.track() {
		conn.Close()
		return
	}
	defer s.sessions.Done()
	s.handle(conn, o, false)
}

func (s *Server) identify(h hello, hash string, host netip.Addr, account *WebAccount) (identity, []byte) {
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
	if account != nil {
		return s.identifyAccount(h, account)
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
	if s.cfg.ReservedName != nil && s.cfg.ReservedName(names.Key(h.name)) {
		return identity{}, mumble.RejectFrame(pb.Reject_UsernameInUse, "This name belongs to an account. Log in to use it.")
	}
	if s.cfg.Password != "" && subtle.ConstantTimeCompare([]byte(h.password), []byte(s.cfg.Password)) != 1 {
		s.logins.failed(host)
		return identity{}, mumble.RejectFrame(pb.Reject_WrongServerPW, "Wrong server password.")
	}
	return identity{name: h.name, hash: hash, tokens: h.tokens}, nil
}

// identifyAccount: a Kallisti account is a registered user, made on its
// first visit. Its name must be the account's.
func (s *Server) identifyAccount(h hello, a *WebAccount) (identity, []byte) {
	if names.Key(h.name) != names.Key(a.Name) {
		return identity{}, mumble.RejectFrame(pb.Reject_InvalidUsername, "Use your account name, "+a.Name+".")
	}
	db := s.cfg.DB
	if db == nil || a.CertHash == "" {
		return identity{}, mumble.RejectFrame(pb.Reject_AuthenticatorFail, "This server does not register users.")
	}
	fail := func(err error) (identity, []byte) {
		slog.Error("registering a web account", "account", a.Name, "err", err)
		return identity{}, mumble.RejectFrame(pb.Reject_AuthenticatorFail, "Please try again later.")
	}
	s.regMu.Lock()
	defer s.regMu.Unlock()
	r, err := db.UserByHash(a.CertHash)
	if err != nil {
		return fail(err)
	}
	if r == nil && s.cfg.WebAccountExists != nil && !s.cfg.WebAccountExists(a.Name, a.CertHash) {
		return identity{}, mumble.RejectFrame(pb.Reject_AuthenticatorFail, "This guest account has expired. Please join again.")
	}
	if r == nil {
		// A Mumble user with this name but another certificate: an account
		// must not take over a registration (names are kept apart from now
		// on, see Config.ReservedName).
		if other, err := db.UserByName(a.Name); err != nil {
			return fail(err)
		} else if other != nil {
			slog.Warn("a web account's name is registered by a Mumble user", "account", a.Name, "user", other.ID)
			return identity{}, mumble.RejectFrame(pb.Reject_UsernameInUse,
				"A Mumble user has registered this name. Please ask an admin.")
		}
		id, err := db.RegisterUser(a.Name, a.CertHash)
		if err != nil {
			return fail(err)
		}
		r = &Registered{ID: id, Name: a.Name}
	} else if names.Key(r.Name) != names.Key(a.Name) {
		// An admin renamed the registration: the account keeps its name.
		return identity{}, mumble.RejectFrame(pb.Reject_InvalidUsername,
			"Your Mumble registration is named "+r.Name+" now. Please ask an admin.")
	}
	comment, texture, err := db.UserContent(r.ID)
	if err != nil {
		slog.Error("loading a comment", "user", r.ID, "err", err)
	}
	return identity{name: r.Name, registered: true, userID: r.ID, hash: a.CertHash, tokens: h.tokens,
		lastChannel: r.LastChannel, comment: comment, texture: texture}, nil
}

// checkPassword runs argon2 with the limit on parallel hashes.
func checkPassword(encoded, pw string) bool {
	defer password.Limit()()
	return password.Check(encoded, pw)
}

// writeReject sends a Reject before the connection closes. The client may
// still be writing (on a net.Pipe that would block both sides): its bytes
// are read and dropped meanwhile.
func writeReject(conn net.Conn, reject []byte) {
	go io.Copy(io.Discard, conn)
	conn.SetWriteDeadline(time.Now().Add(flushTimeout))
	conn.Write(reject)
}

// handle serves a connection. counted: Serve counted it in conns.
func (s *Server) handle(conn net.Conn, o Origin, counted bool) {
	defer conn.Close()
	pending := counted // counted by Serve until it authenticates
	if counted {
		defer func() { s.conns.remove(o.Host, pending) }()
	}
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	h, reject, err := handshake(conn)
	if err != nil {
		return
	}
	host := o.Host
	hash := certHash(conn)
	if o.Account != nil {
		hash = o.Account.CertHash
	}
	id := identity{}
	if reject == nil {
		id, reject = s.identify(h, hash, host, o.Account)
	}
	if reject != nil {
		writeReject(conn, reject)
		return
	}
	key, clientNonce, serverNonce := random16(), random16(), random16()
	c := &client{
		conn:          conn,
		out:           make(chan []byte, queueSize),
		name:          id.name,
		limiter:       rate.NewLimiter(messageLimit, messageBurst),
		bandwidth:     rate.NewLimiter(maxBandwidth/8, maxBandwidth/8),
		targetLimiter: rate.NewLimiter(targetLimit, targetBurst),
		crypt:         cryptstate.New(key, serverNonce, clientNonce),
		lastGood:      time.Now(),
		host:          host,
		web:           !counted,
		legacy:        h.legacy,
	}
	setup := &pb.CryptSetup{Key: key[:], ClientNonce: clientNonce[:], ServerNonce: serverNonce[:]}
	if reject := s.join(c, id, setup); reject != nil {
		writeReject(conn, reject)
		return
	}
	if pending {
		s.conns.authenticated()
		pending = false
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
	if len(s.state.Users) >= s.maxUsers() && ghost == 0 && !(id.registered && id.userID == 0) {
		return mumble.RejectFrame(pb.Reject_ServerFull, fmt.Sprintf("The server is full (at most %d users).", s.maxUsers()))
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
	// The last channel if the user may enter it, else root (as Murmur). Not
	// where root is a lobby (no voice in root): there users join voice on
	// purpose.
	lobby := !s.state.Has(u, 0, Speak)
	if ch := s.state.Channels[id.lastChannel]; ch != nil && !lobby && s.state.Has(u, ch.ID, Enter) {
		u.Channel = ch.ID
	}
	if !s.state.Has(u, u.Channel, Enter) {
		return mumble.RejectFrame(pb.Reject_None, "You may not enter this server.")
	}
	s.state.Refresh(u)
	c.send(s.state.snapshot(u), false)
	s.state.AddUser(u)
	s.clients[c.session] = c
	if s.byHost[c.host] == nil {
		s.byHost[c.host] = map[*client]bool{}
	}
	s.byHost[c.host][c] = true
	s.broadcastLocked(userState(u), nil)
	c.sendMsg(mumble.TypeServerSync, &pb.ServerSync{
		Session: proto.Uint32(c.session), MaxBandwidth: proto.Uint32(maxBandwidth),
		WelcomeText: proto.String(s.cfg.Welcome), Permissions: proto.Uint64(s.state.rootPerms(u)),
	})
	c.sendMsg(mumble.TypeServerConfig, &pb.ServerConfig{
		AllowHtml: proto.Bool(false), MessageLength: proto.Uint32(maxTextLength),
		ImageMessageLength: proto.Uint32(maxTextLength), MaxUsers: proto.Uint32(uint32(s.maxUsers())),
	})
	return nil
}

// snapshot is the channels and users a joining user gets, as one queue
// entry: as single frames a big server would fill the queue.
func (s *State) snapshot(joining *User) []byte {
	var out []byte
	tree := s.Tree()
	for _, ch := range tree {
		out = append(out, mumble.MessageFrame(mumble.TypeChannelState, s.channelStateFor(ch, joining))...)
	}
	// Links once all channels exist, as Murmur does.
	for _, ch := range tree {
		if len(ch.Links) > 0 {
			out = append(out, mumble.MessageFrame(mumble.TypeChannelState,
				&pb.ChannelState{ChannelId: proto.Uint32(ch.ID), Links: slices.Sorted(maps.Keys(ch.Links))})...)
		}
	}
	for _, u := range s.Users {
		out = append(out, userState(u)...)
	}
	return out
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
	if delete(s.byHost[c.host], c); len(s.byHost[c.host]) == 0 {
		delete(s.byHost, c.host)
	}
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
	m := &pb.UserState{Session: proto.Uint32(u.Session), Name: proto.String(u.Name), ChannelId: proto.Uint32(u.Channel)}
	// Only flags that are set: clients read every field as a change.
	flag := func(on bool) *bool {
		if on {
			return proto.Bool(true)
		}
		return nil
	}
	m.Mute, m.Deaf, m.Suppress = flag(u.Mute), flag(u.Deaf), flag(u.Suppress)
	m.SelfMute, m.SelfDeaf = flag(u.SelfMute), flag(u.SelfDeaf)
	m.PrioritySpeaker, m.Recording = flag(u.PrioritySpeaker), flag(u.Recording)
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
