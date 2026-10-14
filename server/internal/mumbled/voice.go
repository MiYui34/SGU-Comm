package mumbled

import (
	"net"
	"net/netip"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

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
