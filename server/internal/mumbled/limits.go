package mumbled

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"

	"google.golang.org/protobuf/encoding/protowire"
	"kallisti.chat/server/internal/clientip"
)

const (
	defaultMaxUsers = 100
	// Open TLS handshakes (before Authenticate), against connection floods.
	maxPending = 500
	// Server-list pings get replies without a session: a global limit keeps
	// them from being used for reflection.
	pingsPerSecond = 2000
)

// conns counts connections per IP and open handshakes.
type conns struct {
	mu      sync.Mutex
	perIP   map[netip.Addr]int
	pending int
}

// add counts a new connection; maxPerIP 0 means no per-IP limit.
func (c *conns) add(ip netip.Addr, maxPerIP int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.perIP == nil {
		c.perIP = map[netip.Addr]int{}
	}
	key := clientip.Prefix(ip)
	if (maxPerIP > 0 && c.perIP[key] >= maxPerIP) || c.pending >= maxPending {
		return false
	}
	c.perIP[key]++
	c.pending++
	return true
}

func (c *conns) authenticated() {
	c.mu.Lock()
	c.pending--
	c.mu.Unlock()
}

func (c *conns) remove(ip netip.Addr, pending bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := clientip.Prefix(ip)
	if c.perIP[key]--; c.perIP[key] <= 0 {
		delete(c.perIP, key)
	}
	if pending {
		c.pending--
	}
}

func (s *Server) maxUsers() int {
	if s.cfg.MaxUsers > 0 {
		return s.cfg.MaxUsers
	}
	return defaultMaxUsers
}

// serverPing answers the unencrypted pings of server lists: the legacy
// 12-byte form and the protobuf form with request_extended_information.
// It reports whether the packet was such a ping.
func (s *Server) serverPing(conn *net.UDPConn, addr netip.AddrPort, packet []byte) bool {
	legacy := len(packet) == 12 && binary.BigEndian.Uint32(packet) == 0
	var timestamp uint64
	if !legacy {
		ts, extended, ok := parsePing(packet[min(1, len(packet)):])
		if len(packet) < 2 || packet[0] != 1 || !ok || !extended {
			return false
		}
		timestamp = ts
	}
	if !s.pings.Allow() {
		return true // a ping, but no reply now
	}
	var reply []byte
	switch {
	case legacy:
		users := s.userCount()
		reply = binary.BigEndian.AppendUint32(nil, versionV1)
		reply = append(reply, packet[4:12]...)
		reply = binary.BigEndian.AppendUint32(reply, uint32(users))
		reply = binary.BigEndian.AppendUint32(reply, uint32(s.maxUsers()))
		reply = binary.BigEndian.AppendUint32(reply, maxBandwidth)
	default:
		reply = []byte{1}
		for _, f := range []struct {
			num protowire.Number
			v   uint64
		}{{1, timestamp}, {3, versionV2}, {4, uint64(s.userCount())}, {5, uint64(s.maxUsers())}, {6, maxBandwidth}} {
			reply = protowire.AppendTag(reply, f.num, protowire.VarintType)
			reply = protowire.AppendVarint(reply, f.v)
		}
	}
	conn.WriteToUDPAddrPort(reply, addr)
	return true
}

// parsePing reads an unencrypted protobuf ping. Anything else in it (for
// example an encrypted packet that happens to start with 1) is refused.
func parsePing(b []byte) (timestamp uint64, extended, ok bool) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 || typ != protowire.VarintType || (num != 1 && num != 2) {
			return 0, false, false
		}
		v, m := protowire.ConsumeVarint(b[n:])
		if m < 0 {
			return 0, false, false
		}
		if num == 1 {
			timestamp = v
		} else {
			extended = v != 0
		}
		b = b[n+m:]
	}
	return timestamp, extended, true
}

func (s *Server) userCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.state.Users)
}
