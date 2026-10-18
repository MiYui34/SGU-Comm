package mumbled

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

// plainUDP sends one unencrypted packet and returns the reply.
func plainUDP(t *testing.T, addr string, packet []byte) []byte {
	t.Helper()
	raddr, _ := net.ResolveUDPAddr("udp", addr)
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.Write(packet)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("no reply: %v", err)
	}
	return buf[:n]
}

func TestServerListPing(t *testing.T) {
	addr := startWith(t, "127.0.0.1", Config{MaxUsers: 50})
	dial(t, addr, "alice")
	ident := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	reply := plainUDP(t, addr, append([]byte{0, 0, 0, 0}, ident...))
	if len(reply) != 24 || binary.BigEndian.Uint32(reply) != 0x010500 || !bytes.Equal(reply[4:12], ident) {
		t.Fatalf("reply %x", reply)
	}
	users, max, bw := binary.BigEndian.Uint32(reply[12:]), binary.BigEndian.Uint32(reply[16:]), binary.BigEndian.Uint32(reply[20:])
	if users != 1 || max != 50 || bw != maxBandwidth {
		t.Fatalf("users %d max %d bandwidth %d", users, max, bw)
	}
}

func TestExtendedPing(t *testing.T) {
	addr := startWith(t, "127.0.0.1", Config{MaxUsers: 50})
	req := []byte{1}
	req = protowire.AppendTag(req, 1, protowire.VarintType)
	req = protowire.AppendVarint(req, 777)
	req = protowire.AppendTag(req, 2, protowire.VarintType)
	req = protowire.AppendVarint(req, 1)
	reply := plainUDP(t, addr, req)
	if reply[0] != 1 {
		t.Fatalf("reply %x", reply)
	}
	fields := map[protowire.Number]uint64{}
	b := reply[1:]
	for len(b) > 0 {
		num, _, n := protowire.ConsumeTag(b)
		v, m := protowire.ConsumeVarint(b[n:])
		fields[num], b = v, b[n+m:]
	}
	if fields[1] != 777 || fields[3] != versionV2 || fields[5] != 50 || fields[6] != maxBandwidth {
		t.Fatalf("fields %v", fields)
	}
}

func TestServerFull(t *testing.T) {
	db := testDB(t)
	addr := startWith(t, "127.0.0.1", Config{MaxUsers: 1, DB: db})
	dial(t, addr, "alice")
	c, _ := tryDial(t, addr, "bob")
	var r pb.Reject
	proto.Unmarshal(c.expect(mumble.TypeReject), &r)
	if r.GetType() != pb.Reject_ServerFull {
		t.Fatalf("reject %v", &r)
	}
}

func TestConnectionsPerIP(t *testing.T) {
	addr := startWith(t, "127.0.0.1", Config{MaxPerIP: 2})
	dial(t, addr, "alice")
	dial(t, addr, "bob")
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err == nil {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err = conn.Read(make([]byte, 1))
		conn.Close()
	}
	if err == nil {
		t.Fatal("a third connection from one IP was served")
	}
}

func TestConnectionCounts(t *testing.T) {
	var c conns
	a := netip.MustParseAddr("10.0.0.1")
	if !c.add(a, 2) || !c.add(a, 2) || c.add(a, 2) {
		t.Fatal("per-IP limit")
	}
	c.authenticated()
	c.remove(a, false)
	c.remove(a, true)
	if len(c.perIP) != 0 || c.pending != 0 {
		t.Fatalf("left over: %v pending %d", c.perIP, c.pending)
	}
	// One IPv6 network counts as one address.
	x, y := netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("2001:db8::2")
	if !c.add(x, 1) || c.add(y, 1) {
		t.Fatal("a /64 is one address")
	}
	// No per-IP limit (0): only the handshake cap.
	var d conns
	for i := range maxPending {
		if !d.add(netip.AddrFrom4([4]byte{10, 1, byte(i >> 8), byte(i)}), 0) {
			t.Fatalf("refused connection %d", i)
		}
	}
	if d.add(netip.MustParseAddr("10.9.9.9"), 0) {
		t.Fatal("more open handshakes than the cap")
	}
}

func TestLoginFailuresCountIPv6Slash64s(t *testing.T) {
	var l logins
	for range maxLoginFailures {
		l.failed(netip.MustParseAddr("2001:db8:1:2::1"))
	}
	if !l.blocked(netip.MustParseAddr("2001:db8:1:2::2")) {
		t.Error("the same /64 is not blocked")
	}
	if l.blocked(netip.MustParseAddr("2001:db8:1:3::1")) {
		t.Error("another /64 is blocked")
	}
	for range maxLoginFailures {
		l.failed(netip.MustParseAddr("192.0.2.1"))
	}
	if l.blocked(netip.MustParseAddr("192.0.2.2")) {
		t.Error("another IPv4 address is blocked")
	}
}

func TestParsePingRefusesOtherBytes(t *testing.T) {
	for _, b := range [][]byte{{0x0a, 0x01, 0x00}, {0x08}, {0x18, 0x01}} {
		if _, _, ok := parsePing(b); ok {
			t.Errorf("accepted %x", b)
		}
	}
}

func TestSuperUserJoinsAFullServer(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	addr := startWith(t, "127.0.0.1", Config{MaxUsers: 1, DB: db})
	dial(t, addr, "alice")
	superUser(t, addr)
}

func TestVoiceTargetsAreRateLimited(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, openACL)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	s.mu.Lock()
	limiter := s.clients[alice.session].targetLimiter
	s.mu.Unlock()
	limiter.SetLimit(0)
	limiter.SetBurst(0) // empty, and it does not fill up
	alice.send(mumble.TypeVoiceTarget, &pb.VoiceTarget{Id: proto.Uint32(1), Targets: []*pb.VoiceTarget_Target{{Session: []uint32{bob.session}}}})
	alice.send(mumble.TypePing, &pb.Ping{})
	alice.expect(mumble.TypePing) // the target came first
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.state.Users[alice.session].Targets); n != 0 {
		t.Fatalf("%d targets", n)
	}
}
