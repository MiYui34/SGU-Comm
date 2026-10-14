package mumbled

import (
	"net"
	"net/netip"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// splitAudio splits a protobuf audio packet into its voice target and
// the fields that are passed on unchanged. ok is false for packets that
// are not audio.
func splitAudio(packet []byte) (target uint32, rest []byte, ok bool) {
	if len(packet) == 0 || packet[0] != 0 {
		return 0, nil, false
	}
	b := packet[1:]
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return 0, nil, false
		}
		m := protowire.ConsumeFieldValue(num, typ, b[n:])
		if m < 0 {
			return 0, nil, false
		}
		switch num {
		case 1:
			v, _ := protowire.ConsumeVarint(b[n:])
			if typ != protowire.VarintType || v > loopbackTarget {
				return 0, nil, false
			}
			target = uint32(v)
		case 2, 3: // set by the server
		default:
			rest = append(rest, b[:n+m]...)
		}
		b = b[n+m:]
	}
	return target, rest, true
}

// relayed is the audio packet as a receiver gets it: the server sets the
// context and the sender's session.
func relayed(rest []byte, sender uint32, ctx AudioContext) []byte {
	out := []byte{0}
	out = protowire.AppendTag(out, 2, protowire.VarintType)
	out = protowire.AppendVarint(out, uint64(ctx))
	out = protowire.AppendTag(out, 3, protowire.VarintType)
	out = protowire.AppendVarint(out, uint64(sender))
	return append(out, rest...)
}

// voice relays an audio packet from c to its receivers.
func (s *Server) voice(c *client, packet []byte) {
	target, rest, ok := splitAudio(packet)
	// As Murmur: headers count too (IP, UDP, crypt).
	if !ok || !c.bandwidth.AllowN(time.Now(), 20+8+4+len(packet)) {
		return
	}
	type receiver struct {
		c    *client
		addr netip.AddrPort
		ctx  AudioContext
	}
	s.mu.Lock()
	var receivers []receiver
	for session, ctx := range s.state.Receivers(c.session, target) {
		o := s.clients[session]
		if o == nil {
			continue
		}
		r := receiver{c: o, ctx: ctx}
		if o.udpVoice && s.udp != nil {
			r.addr = o.udpAddr
		}
		receivers = append(receivers, r)
	}
	udp := s.udp
	s.mu.Unlock()
	// One packet (and TCP frame) per context, at most four.
	type packetFor struct{ udp, tcp []byte }
	packets := map[AudioContext]packetFor{}
	for _, r := range receivers {
		p, ok := packets[r.ctx]
		if !ok {
			p.udp = relayed(rest, c.session, r.ctx)
			p.tcp = mumble.EncodeFrame(mumble.TypeUDPTunnel, p.udp)
			packets[r.ctx] = p
		}
		if r.addr.IsValid() {
			r.c.cryptMu.Lock()
			crypted := r.c.crypt.Encrypt(p.udp)
			r.c.cryptMu.Unlock()
			udp.WriteToUDPAddrPort(crypted, r.addr)
		} else {
			r.c.send(p.tcp, true)
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
