package mumble

import (
	"encoding/binary"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// AuthReader reads the client-to-server stream until the Authenticate
// message and returns the user name the client asks for.
type AuthReader struct {
	frames frameReader
	name   string
	done   bool
}

func (a *AuthReader) Feed(chunk []byte) (name string, done bool, err error) {
	if a.done {
		return a.name, true, nil
	}
	err = a.frames.feed(chunk,
		func(typ uint16) bool { return typ == TypeAuthenticate && !a.done },
		func(_ uint16, body []byte) {
			var m pb.Authenticate
			if proto.Unmarshal(body, &m) == nil {
				a.name, a.done = m.GetUsername(), true
			}
		})
	return a.name, a.done, err
}

// RejectFrame is a Mumble Reject message, as Murmur would send it.
func RejectFrame(typ pb.Reject_RejectType, reason string) []byte {
	body, _ := proto.Marshal(&pb.Reject{Type: typ.Enum(), Reason: proto.String(reason)})
	out := make([]byte, headerSize, headerSize+len(body))
	binary.BigEndian.PutUint16(out, TypeReject)
	binary.BigEndian.PutUint32(out[2:], uint32(len(body)))
	return append(out, body...)
}
