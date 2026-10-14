package mumble

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

func clientHello(t *testing.T, name string) []byte {
	var b []byte
	b = append(b, frame(t, TypeVersion, &pb.Version{Release: proto.String("Kallisti")})...)
	b = append(b, frame(t, TypeAuthenticate, &pb.Authenticate{Username: proto.String(name), Opus: proto.Bool(true)})...)
	return b
}

func TestAuthReaderFindsTheName(t *testing.T) {
	var r AuthReader
	name, done, err := r.Feed(clientHello(t, "alice"))
	if err != nil || !done || name != "alice" {
		t.Fatalf("got %q %v %v", name, done, err)
	}
}

func TestAuthReaderAcrossChunks(t *testing.T) {
	var r AuthReader
	hello := clientHello(t, "bob")
	for i, b := range hello {
		name, done, err := r.Feed([]byte{b})
		if err != nil {
			t.Fatal(err)
		}
		if done != (i == len(hello)-1) {
			t.Fatalf("done=%v at byte %d of %d", done, i, len(hello))
		}
		if done && name != "bob" {
			t.Fatalf("name %q", name)
		}
	}
}

func TestRejectFrame(t *testing.T) {
	var s Sniffer
	f := RejectFrame(pb.Reject_UsernameInUse, "name in use")
	if !bytes.Equal(f[:2], []byte{0, TypeReject}) {
		t.Fatalf("type bytes %v", f[:2])
	}
	var m pb.Reject
	if err := proto.Unmarshal(f[6:], &m); err != nil || m.GetType() != pb.Reject_UsernameInUse || m.GetReason() != "name in use" {
		t.Fatalf("got %v %v", &m, err)
	}
	_ = s
}
