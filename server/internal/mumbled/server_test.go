package mumbled

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/cryptstate"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func start(t *testing.T) string {
	t.Helper()
	return startOn(t, "127.0.0.1")
}

func startOn(t *testing.T, host string) string {
	t.Helper()
	return startWith(t, host, Config{Welcome: "Welcome!"})
}

func startWith(t *testing.T, host string, cfg Config) string {
	t.Helper()
	return startServer(t, host, cfg, nil)
}

// startOpen gives everyone Write in the root channel: for tests about
// other things than permissions.
func startOpen(t *testing.T) string {
	t.Helper()
	return startServer(t, "127.0.0.1", Config{Welcome: "Welcome!"}, openACL)
}

func openACL(s *Server) {
	s.state.Channels[0].ACLs = append(s.state.Channels[0].ACLs, ACL{ApplyHere: true, ApplySubs: true, Group: "all", Allow: Write})
}

func startServer(t *testing.T, host string, cfg Config, setup func(*Server)) string {
	t.Helper()
	certPEM, keyPEM, err := certs.GenerateServer("test")
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := certs.Load(certPEM, keyPEM)
	ln, err := tls.Listen("tcp", net.JoinHostPort(host, "0"), TLSConfig(cert))
	if err != nil {
		t.Fatal(err)
	}
	udp, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(ln.Addr().(*net.TCPAddr).AddrPort()))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(s)
	}
	go s.Serve(ln)
	go s.ServeUDP(udp)
	t.Cleanup(func() {
		ln.Close()
		udp.Close()
		s.Close()
	})
	return ln.Addr().String()
}

type frame struct {
	typ  uint16
	body []byte
}

type testClient struct {
	t       *testing.T
	conn    *tls.Conn
	frames  chan frame
	session uint32
	crypt   *cryptstate.CryptState
	udp     *net.UDPConn
}

func dial(t *testing.T, addr, name string) *testClient {
	t.Helper()
	c, err := tryDial(t, addr, name)
	if err != nil {
		t.Fatal(err)
	}
	sync := c.expect(mumble.TypeServerSync)
	var m pb.ServerSync
	proto.Unmarshal(sync, &m)
	c.session = m.GetSession()
	return c
}

// tryDial connects with a client certificate, as Mumble clients do.
func tryDial(t *testing.T, addr, name string) (*testClient, error) {
	return tryDialWith(t, addr, dialOpts{name: name, cert: clientCert(t)})
}

func tryDialVersion(t *testing.T, addr, name string, major, minor uint64) (*testClient, error) {
	return tryDialWith(t, addr, dialOpts{name: name, major: major, minor: minor})
}

type dialOpts struct {
	name, password string
	cert           *tls.Certificate
	major, minor   uint64 // default 1.5
}

// clientCert makes a client certificate, as Mumble clients have.
func clientCert(t *testing.T) *tls.Certificate {
	t.Helper()
	certPEM, keyPEM, err := certs.Generate("client")
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := certs.Load(certPEM, keyPEM)
	return &cert
}

// dialAs connects and waits for ServerSync.
func dialAs(t *testing.T, addr string, o dialOpts) *testClient {
	t.Helper()
	c, err := tryDialWith(t, addr, o)
	if err != nil {
		t.Fatal(err)
	}
	var m pb.ServerSync
	proto.Unmarshal(c.expect(mumble.TypeServerSync), &m)
	c.session = m.GetSession()
	return c
}

func tryDialWith(t *testing.T, addr string, o dialOpts) (*testClient, error) {
	if o.major == 0 {
		o.major, o.minor = 1, 5
	}
	major, minor, name := o.major, o.minor, o.name
	cfg := &tls.Config{InsecureSkipVerify: true}
	if o.cert != nil {
		cfg.Certificates = []tls.Certificate{*o.cert}
	}
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	c := &testClient{t: t, conn: conn, frames: make(chan frame, 256)}
	t.Cleanup(func() { conn.Close() })
	go func() {
		defer close(c.frames)
		header := make([]byte, 6)
		for {
			if _, err := io.ReadFull(conn, header); err != nil {
				return
			}
			body := make([]byte, binary.BigEndian.Uint32(header[2:]))
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
			typ := binary.BigEndian.Uint16(header)
			if typ == mumble.TypeCryptSetup {
				var m pb.CryptSetup
				proto.Unmarshal(body, &m)
				if len(m.Key) == 16 {
					c.crypt = cryptstate.New([16]byte(m.Key), [16]byte(m.ClientNonce), [16]byte(m.ServerNonce))
				}
			}
			c.frames <- frame{typ, body}
		}
	}()
	v1, v2 := uint32(major<<16|minor<<8), major<<48|minor<<32
	c.send(mumble.TypeVersion, &pb.Version{VersionV1: &v1, VersionV2: &v2, Release: proto.String("test")})
	c.send(mumble.TypeAuthenticate, &pb.Authenticate{Username: proto.String(name), Password: proto.String(o.password), Opus: proto.Bool(true)})
	return c, nil
}

func (c *testClient) send(typ uint16, m proto.Message) {
	c.conn.Write(mumble.MessageFrame(typ, m))
}

// expect reads frames until one of the type arrives.
func (c *testClient) expect(typ uint16) []byte {
	c.t.Helper()
	return c.expectWithin(typ, 3*time.Second)
}

func (c *testClient) expectWithin(typ uint16, d time.Duration) []byte {
	c.t.Helper()
	timeout := time.After(d)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				c.t.Fatalf("connection closed, waiting for type %d", typ)
			}
			if f.typ == typ {
				return f.body
			}
		case <-timeout:
			c.t.Fatalf("no frame of type %d", typ)
		}
	}
}

// expectNone fails if a frame of the type arrives within a short time.
func (c *testClient) expectNone(typ uint16) {
	c.t.Helper()
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case f, ok := <-c.frames:
			if ok && f.typ == typ {
				c.t.Fatalf("unexpected frame of type %d", typ)
			}
		case <-timeout:
			return
		}
	}
}

// audio is a protobuf UDP audio packet (type byte 0) to the channel.
func audio(frameNumber uint64, opus []byte) []byte {
	b := []byte{0}
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 0)
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, frameNumber)
	b = protowire.AppendTag(b, 5, protowire.BytesType)
	return protowire.AppendBytes(b, opus)
}

type heard struct {
	sender uint32
	frame  uint64
	opus   []byte
}

func parseAudio(t *testing.T, packet []byte) heard {
	t.Helper()
	if len(packet) == 0 || packet[0] != 0 {
		t.Fatalf("not an audio packet: %x", packet)
	}
	var h heard
	b := packet[1:]
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		b = b[n:]
		switch {
		case num == 3 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			h.sender, b = uint32(v), b[n:]
		case num == 4 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			h.frame, b = v, b[n:]
		case num == 5 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			h.opus, b = v, b[n:]
		default:
			b = b[protowire.ConsumeFieldValue(num, typ, b):]
		}
	}
	return h
}

func TestHandshake(t *testing.T) {
	addr := start(t)
	c, _ := tryDial(t, addr, "alice")
	var version pb.Version
	proto.Unmarshal(c.expect(mumble.TypeVersion), &version)
	if version.GetVersionV2()>>32 != 1<<16|5 {
		t.Fatalf("server version %x", version.GetVersionV2())
	}
	c.expect(mumble.TypeCryptSetup)
	var root pb.ChannelState
	proto.Unmarshal(c.expect(mumble.TypeChannelState), &root)
	if root.GetChannelId() != 0 || root.GetName() == "" {
		t.Fatalf("root channel %v", &root)
	}
	var self pb.UserState
	proto.Unmarshal(c.expect(mumble.TypeUserState), &self)
	if self.GetName() != "alice" {
		t.Fatalf("user state %v", &self)
	}
	var sync pb.ServerSync
	proto.Unmarshal(c.expect(mumble.TypeServerSync), &sync)
	if sync.GetSession() != self.GetSession() || sync.GetWelcomeText() != "Welcome!" {
		t.Fatalf("server sync %v", &sync)
	}
	if c.crypt == nil {
		t.Fatal("no crypt keys")
	}
}

func TestUsersSeeEachOtherComeAndGo(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	var joined pb.UserState
	proto.Unmarshal(alice.expect(mumble.TypeUserState), &joined)
	if joined.GetName() != "bob" || joined.GetSession() != bob.session {
		t.Fatalf("alice saw %v", &joined)
	}
	bob.conn.Close()
	var removed pb.UserRemove
	proto.Unmarshal(alice.expect(mumble.TypeUserRemove), &removed)
	if removed.GetSession() != bob.session {
		t.Fatalf("removed %v", &removed)
	}
}

func TestNewUsersGetTheUserList(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	c, _ := tryDial(t, addr, "bob")
	names := map[string]bool{}
	for len(names) < 2 {
		var u pb.UserState
		proto.Unmarshal(c.expect(mumble.TypeUserState), &u)
		names[u.GetName()] = true
	}
	_ = alice
}

func TestInvalidNamesAreRefused(t *testing.T) {
	addr := start(t)
	c, _ := tryDial(t, addr, "  ")
	var r pb.Reject
	proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_InvalidUsername {
		t.Fatalf("reject %v", &r)
	}
}

func TestAGuestFromTheSameIPReplacesItsOldSession(t *testing.T) {
	// As Murmur: a reconnect after a network drop must not wait for the
	// old session to time out.
	addr := start(t)
	old := dial(t, addr, "alice")
	watcher := dial(t, addr, "watcher")
	dial(t, addr, "Alice")
	var m pb.UserRemove
	proto.Unmarshal(old.expect(mumble.TypeUserRemove), &m)
	if m.GetSession() != old.session {
		t.Fatalf("old session got %v", &m)
	}
	for {
		proto.Unmarshal(watcher.expect(mumble.TypeUserRemove), &m)
		if m.GetSession() == old.session {
			break
		}
	}
}

func TestOldClientsAreRefused(t *testing.T) {
	// The spike only speaks the 1.5 UDP format.
	c, err := tryDialVersion(t, start(t), "old", 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	var r pb.Reject
	proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_WrongVersion {
		t.Fatalf("reject %v", &r)
	}
}

func TestPingsAreAnswered(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	alice.send(mumble.TypePing, &pb.Ping{Timestamp: proto.Uint64(1234)})
	var pong pb.Ping
	proto.Unmarshal(alice.expect(mumble.TypePing), &pong)
	if pong.GetTimestamp() != 1234 {
		t.Fatalf("pong %v", &pong)
	}
}

func TestVoiceOverTCPReachesOthers(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(7, []byte{0xfc, 1, 2})))
	h := parseAudio(t, bob.expect(mumble.TypeUDPTunnel))
	if h.sender != alice.session || h.frame != 7 || !bytes.Equal(h.opus, []byte{0xfc, 1, 2}) {
		t.Fatalf("bob heard %+v", h)
	}
	alice.expectNone(mumble.TypeUDPTunnel)
}

func TestDeafenedUsersHearNothing(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	bob.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(bob.session), SelfDeaf: proto.Bool(true)})
	var state pb.UserState
	for state.GetSession() != bob.session || !state.GetSelfDeaf() {
		proto.Unmarshal(alice.expect(mumble.TypeUserState), &state)
	}
	alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(1, []byte{0xfc})))
	bob.expectNone(mumble.TypeUDPTunnel)
}

func (c *testClient) openUDP(addr string) {
	c.t.Helper()
	raddr, _ := net.ResolveUDPAddr("udp", addr)
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { conn.Close() })
	c.udp = conn
}

func (c *testClient) sendUDP(plain []byte) {
	c.udp.Write(c.crypt.Encrypt(plain))
}

func (c *testClient) readUDP() []byte {
	c.t.Helper()
	buf := make([]byte, 2048)
	c.udp.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := c.udp.Read(buf)
	if err != nil {
		c.t.Fatalf("no UDP packet: %v", err)
	}
	plain, ok := c.crypt.Decrypt(buf[:n])
	if !ok {
		c.t.Fatal("UDP packet does not decrypt")
	}
	return plain
}

func TestVoiceOverUDP(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.openUDP(addr)
	bob.openUDP(addr)
	// UDP pings come back over UDP.
	alice.sendUDP([]byte{1, 0x08, 42})
	if pong := alice.readUDP(); !bytes.Equal(pong, []byte{1, 0x08, 42}) {
		t.Fatalf("pong %x", pong)
	}
	// Like Murmur: a client gets voice over UDP once it sent voice over UDP.
	bob.sendUDP(audio(1, []byte{0xfc}))
	alice.expect(mumble.TypeUDPTunnel) // alice only pinged: TCP
	alice.sendUDP(audio(9, []byte{0xfc, 9}))
	h := parseAudio(t, bob.readUDP())
	if h.sender != alice.session || h.frame != 9 {
		t.Fatalf("bob heard %+v", h)
	}
	bob.expectNone(mumble.TypeUDPTunnel)
	// Voice over TCP switches the client back to TCP.
	bob.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(2, []byte{0xfc})))
	alice.readUDP()
	alice.sendUDP(audio(10, []byte{0xfc}))
	bob.expect(mumble.TypeUDPTunnel)
}

func TestStatsCountDecryptedPackets(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	alice.openUDP(addr)
	alice.sendUDP([]byte{1, 0x08, 1})
	alice.readUDP()
	alice.send(mumble.TypeUserStats, &pb.UserStats{Session: proto.Uint32(alice.session)})
	var stats pb.UserStats
	proto.Unmarshal(alice.expect(mumble.TypeUserStats), &stats)
	if stats.GetFromClient().GetGood() != 1 {
		t.Fatalf("stats %v", &stats)
	}
}

func TestCryptResync(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	// The client asks for the server nonce.
	alice.send(mumble.TypeCryptSetup, &pb.CryptSetup{})
	var m pb.CryptSetup
	proto.Unmarshal(alice.expect(mumble.TypeCryptSetup), &m)
	if len(m.ServerNonce) != 16 || len(m.Key) != 0 {
		t.Fatalf("resync %v", &m)
	}
}

func TestHTMLIsRemovedFromText(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String("<b>hi</b> all")})
	var m pb.TextMessage
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if m.GetMessage() != "hi all" || m.GetActor() != alice.session {
		t.Fatalf("bob got %v", &m)
	}
}

func TestTextFloodsAreLimited(t *testing.T) {
	addr := start(t)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	for range 20 {
		alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String("spam")})
	}
	got := 0
	timeout := time.After(500 * time.Millisecond)
	for done := false; !done; {
		select {
		case f := <-bob.frames:
			if f.typ == mumble.TypeTextMessage {
				got++
			}
		case <-timeout:
			done = true
		}
	}
	if got == 0 || got > 6 {
		t.Fatalf("bob got %d of 20 messages", got)
	}
}

func TestServerAsksForResyncWhenPacketsStopDecrypting(t *testing.T) {
	resyncAfter = 0
	t.Cleanup(func() { resyncAfter = 5 * time.Second })
	addr := start(t)
	alice := dial(t, addr, "alice")
	alice.openUDP(addr)
	alice.sendUDP([]byte{1, 0x08, 1}) // the server learns the address
	alice.readUDP()
	alice.expectNone(mumble.TypeCryptSetup)
	// Lost sync: the client's IV jumped far ahead.
	alice.crypt.EncryptIV[3]++
	alice.sendUDP([]byte{1, 0x08, 2})
	var m pb.CryptSetup
	proto.Unmarshal(alice.expect(mumble.TypeCryptSetup), &m)
	if len(m.Key)+len(m.ClientNonce)+len(m.ServerNonce) != 0 {
		t.Fatalf("expected an empty CryptSetup, got %v", &m)
	}
}

func TestSelfMuteAndDeafRules(t *testing.T) {
	for _, c := range []struct {
		mute, deaf       bool
		setMute, setDeaf *bool
		wantMute, wantDf bool
	}{
		{false, false, proto.Bool(true), nil, true, false},
		{false, false, nil, proto.Bool(true), true, true},  // deaf implies mute
		{true, true, proto.Bool(false), nil, false, false}, // unmute undeafens
		{true, true, nil, proto.Bool(false), true, false},  // undeafen keeps mute
	} {
		mute, deaf := selfState(c.mute, c.deaf, &pb.UserState{SelfMute: c.setMute, SelfDeaf: c.setDeaf})
		if mute != c.wantMute || deaf != c.wantDf {
			t.Errorf("%+v: got mute %v deaf %v", c, mute, deaf)
		}
	}
}
