// Package mumble reads the parts of the Mumble protocol the server needs.
package mumble

import (
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// TCP message types (the order of messages in Mumble.proto).
const (
	TypeVersion         = 0
	TypeUDPTunnel       = 1
	TypeAuthenticate    = 2
	TypePing            = 3
	TypeReject          = 4
	TypeServerSync      = 5
	TypeChannelState    = 7
	TypeUserRemove      = 8
	TypeUserState       = 9
	TypeTextMessage     = 11
	TypeCryptSetup      = 15
	TypePermissionQuery = 20
	TypeCodecVersion    = 21
	TypeUserStats       = 22
	TypeServerConfig    = 24
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
	// Registered on the Mumble server: the name belongs to this user. An
	// unregistered name is only unique while its owner is online.
	Registered bool
}

// Sniffer reads the server-to-client byte stream of one connection and
// finds out whose connection it is. It never changes the stream. Frames it
// does not need (voice is most of the traffic) are skipped, not copied.
type Sniffer struct {
	frames frameReader
	synced bool
	self   Identity
	// Users seen before ServerSync: Murmur sends all UserStates first.
	users map[uint32]Identity
}

// Feed takes the next chunk. It returns the identity and whether it changed
// with this chunk (first known after ServerSync, or renamed later).
func (s *Sniffer) Feed(chunk []byte) (Identity, bool, error) {
	changed := false
	err := s.frames.feed(chunk,
		func(typ uint16) bool { return typ == TypeServerSync || typ == TypeUserState },
		func(typ uint16, body []byte) { changed = s.handle(typ, body) || changed })
	return s.self, changed, err
}

// handle processes one complete frame. Broken messages are skipped: the
// stream itself is still in sync, only this message is unreadable.
func (s *Sniffer) handle(typ uint16, body []byte) bool {
	switch typ {
	case TypeUserState:
		var m pb.UserState
		if proto.Unmarshal(body, &m) != nil || m.Session == nil {
			return false
		}
		// UserStates are partial: apply only the fields that are set.
		apply := func(id Identity) Identity {
			if m.Name != nil {
				id.Name = m.GetName()
			}
			if m.UserId != nil {
				id.Registered = true
			}
			return id
		}
		if !s.synced {
			if s.users == nil {
				s.users = map[uint32]Identity{}
			}
			s.users[m.GetSession()] = apply(s.users[m.GetSession()])
			return false
		}
		if m.GetSession() != s.self.Session {
			return false
		}
		next := apply(s.self)
		changed := next != s.self
		s.self = next
		return changed
	case TypeServerSync:
		var m pb.ServerSync
		if s.synced || proto.Unmarshal(body, &m) != nil {
			return false
		}
		s.synced = true
		s.self = s.users[m.GetSession()]
		s.self.Session = m.GetSession()
		s.users = nil
		return true
	}
	return false
}
