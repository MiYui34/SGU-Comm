package mumble

import (
	"encoding/binary"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

func frame(t *testing.T, typ uint16, msg proto.Message) []byte {
	t.Helper()
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return rawFrame(typ, body)
}

func rawFrame(typ uint16, body []byte) []byte {
	out := make([]byte, 6, 6+len(body))
	binary.BigEndian.PutUint16(out, typ)
	binary.BigEndian.PutUint32(out[2:], uint32(len(body)))
	return append(out, body...)
}

func handshake(t *testing.T) []byte {
	var b []byte
	b = append(b, frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(3), Name: proto.String("bob")})...)
	b = append(b, rawFrame(TypeUDPTunnel, make([]byte, 500))...)
	b = append(b, frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), Name: proto.String("alice")})...)
	b = append(b, frame(t, TypeServerSync, &pb.ServerSync{Session: proto.Uint32(7)})...)
	return b
}

func TestIdentityAfterServerSync(t *testing.T) {
	var s Sniffer
	id, changed, err := s.Feed(handshake(t))
	if err != nil || !changed || id != (Identity{Session: 7, Name: "alice"}) {
		t.Fatalf("got %+v %v %v", id, changed, err)
	}
}

func TestNoIdentityBeforeServerSync(t *testing.T) {
	var s Sniffer
	_, changed, _ := s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), Name: proto.String("alice")}))
	if changed {
		t.Fatal("identity reported before ServerSync")
	}
}

func TestChunkBoundaries(t *testing.T) {
	var s Sniffer
	var got Identity
	var seen bool
	for _, b := range handshake(t) {
		id, changed, err := s.Feed([]byte{b})
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			got, seen = id, true
		}
	}
	if !seen || got != (Identity{Session: 7, Name: "alice"}) {
		t.Fatalf("got %+v %v", got, seen)
	}
}

func TestRenameOfOwnSession(t *testing.T) {
	var s Sniffer
	s.Feed(handshake(t))
	id, changed, _ := s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), Name: proto.String("alicia")}))
	if !changed || id.Name != "alicia" {
		t.Fatalf("got %+v %v", id, changed)
	}
	_, changed, _ = s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(3), Name: proto.String("robert")}))
	if changed {
		t.Fatal("rename of another user reported")
	}
	// A UserState without a name (for example a mute change) changes nothing.
	_, changed, _ = s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), SelfMute: proto.Bool(true)}))
	if changed {
		t.Fatal("state change without name reported")
	}
}

func TestRejectsOversizedFrames(t *testing.T) {
	var s Sniffer
	header := make([]byte, 6)
	binary.BigEndian.PutUint32(header[2:], MaxPayload+1)
	if _, _, err := s.Feed(header); err == nil {
		t.Fatal("expected an error")
	}
}

func TestIgnoresBrokenProtobuf(t *testing.T) {
	var s Sniffer
	_, _, err := s.Feed(rawFrame(TypeUserState, []byte{0xff, 0xff, 0xff}))
	if err != nil {
		t.Fatalf("broken message must be skipped, got %v", err)
	}
	if _, changed, _ := s.Feed(handshake(t)); !changed {
		t.Fatal("sniffer stopped after a broken message")
	}
}

func TestRegisteredUsers(t *testing.T) {
	var s Sniffer
	b := frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), Name: proto.String("alice"), UserId: proto.Uint32(12)})
	b = append(b, frame(t, TypeServerSync, &pb.ServerSync{Session: proto.Uint32(7)})...)
	id, _, _ := s.Feed(b)
	if !id.Registered || id.UserID != 12 {
		t.Fatalf("got %+v, want registered as 12", id)
	}
	var u Sniffer
	id, _, _ = u.Feed(handshake(t))
	if id.Registered {
		t.Fatalf("got %+v, want unregistered", id)
	}
}

func TestUnregistrationOnline(t *testing.T) {
	var s Sniffer
	s.Feed(handshake(t))
	s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), UserId: proto.Uint32(3)}))
	// Murmur sends this id when the registration is removed.
	id, changed, _ := s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), UserId: proto.Uint32(0xffffffff)}))
	if !changed || id.Registered || id.UserID != 0 {
		t.Fatalf("got %+v %v, want unregistered", id, changed)
	}
}

func TestRegistrationLater(t *testing.T) {
	var s Sniffer
	s.Feed(handshake(t))
	id, changed, _ := s.Feed(frame(t, TypeUserState, &pb.UserState{Session: proto.Uint32(7), UserId: proto.Uint32(3)}))
	if !changed || !id.Registered || id.UserID != 3 || id.Name != "alice" {
		t.Fatalf("got %+v %v", id, changed)
	}
}
