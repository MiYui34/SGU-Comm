// Package mumble reads the parts of the Mumble protocol the server needs.
package mumble

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// TCP message types (the order of messages in Mumble.proto).
const (
	TypeUDPTunnel  = 1
	TypeServerSync = 5
	TypeUserState  = 9
)

const (
	headerSize = 6
	// MaxPayload is the frame limit of the reference server.
	MaxPayload = 8*1024*1024 - 1
)

// Identity is the Mumble user behind one connection, as verified by Murmur.
type Identity struct {
	Session uint32
	Name    string
}

// Sniffer reads the server-to-client byte stream of one connection and
// finds out whose connection it is. It never changes the stream. Frames it
// does not need (voice is most of the traffic) are skipped, not copied.
type Sniffer struct {
	header []byte
	typ    uint16
	body   []byte
	need   int // body bytes still to read (or skip) for the current frame
	keep   bool

	synced bool
	self   Identity
	// Names seen before ServerSync: Murmur sends all UserStates first.
	names map[uint32]string
}

// Feed takes the next chunk. It returns the identity and whether it changed
// with this chunk (first known after ServerSync, or renamed later).
func (s *Sniffer) Feed(chunk []byte) (Identity, bool, error) {
	changed := false
	for len(chunk) > 0 {
		// need == 0: between frames (empty bodies are handled at once).
		if s.need == 0 {
			n := min(headerSize-len(s.header), len(chunk))
			s.header = append(s.header, chunk[:n]...)
			chunk = chunk[n:]
			if len(s.header) < headerSize {
				break
			}
			s.typ = binary.BigEndian.Uint16(s.header)
			length := binary.BigEndian.Uint32(s.header[2:])
			s.header = s.header[:0]
			if length > MaxPayload {
				return s.self, changed, fmt.Errorf("frame too large: %d bytes", length)
			}
			s.need = int(length)
			s.keep = s.typ == TypeServerSync || s.typ == TypeUserState
			s.body = s.body[:0]
			if s.need == 0 {
				changed = s.handle() || changed
			}
			continue
		}
		n := min(s.need, len(chunk))
		if s.keep {
			s.body = append(s.body, chunk[:n]...)
		}
		s.need -= n
		chunk = chunk[n:]
		if s.need == 0 {
			changed = s.handle() || changed
		}
	}
	return s.self, changed, nil
}

// handle processes one complete frame. Broken messages are skipped: the
// stream itself is still in sync, only this message is unreadable.
func (s *Sniffer) handle() bool {
	if !s.keep {
		return false
	}
	switch s.typ {
	case TypeUserState:
		var m pb.UserState
		if proto.Unmarshal(s.body, &m) != nil || m.Session == nil || m.Name == nil {
			return false
		}
		if !s.synced {
			if s.names == nil {
				s.names = map[uint32]string{}
			}
			s.names[m.GetSession()] = m.GetName()
			return false
		}
		if m.GetSession() == s.self.Session && m.GetName() != s.self.Name {
			s.self.Name = m.GetName()
			return true
		}
	case TypeServerSync:
		var m pb.ServerSync
		if s.synced || proto.Unmarshal(s.body, &m) != nil {
			return false
		}
		s.synced = true
		s.self = Identity{Session: m.GetSession(), Name: s.names[m.GetSession()]}
		s.names = nil
		return true
	}
	return false
}
