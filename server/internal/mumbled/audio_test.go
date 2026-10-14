package mumbled

import (
	"bytes"
	"slices"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestLegacyVarints(t *testing.T) {
	for _, v := range []uint64{0, 1, 0x7f, 0x80, 0x3fff, 0x4000, 0x1fffff, 0x200000, 0xfffffff, 0x10000000, 0xffffffff, 1 << 40} {
		b := appendLegacyVarint(nil, v)
		got, n, ok := readLegacyVarint(b)
		if !ok || got != v || n != len(b) {
			t.Errorf("%d: got %d (%d of %d bytes) ok %v", v, got, n, len(b), ok)
		}
	}
	// Byte layouts from Mumble's PacketDataStream.
	for v, want := range map[uint64][]byte{
		5:       {0x05},
		0x1234:  {0x92, 0x34},
		0x12345: {0xc1, 0x23, 0x45},
	} {
		if got := appendLegacyVarint(nil, v); !bytes.Equal(got, want) {
			t.Errorf("%#x: %x, want %x", v, got, want)
		}
	}
	if _, _, ok := readLegacyVarint([]byte{0xc1, 0x23}); ok {
		t.Error("read a truncated varint")
	}
}

func TestAudioRoundTripsInBothFormats(t *testing.T) {
	p := audioPacket{target: 3, frame: 1234, opus: []byte{0xfc, 1, 2, 3}, terminator: true, positional: []float32{1.5, -2, 3.25}}
	for _, legacy := range []bool{false, true} {
		// Client to server: the target, no session.
		got, ok := decodeAudio(encodeClientAudio(p, legacy), legacy)
		if !ok || got.target != p.target || got.frame != p.frame || !bytes.Equal(got.opus, p.opus) ||
			!got.terminator || !slices.Equal(got.positional, p.positional) {
			t.Errorf("legacy %v: %+v", legacy, got)
		}
	}
}

func TestServerAudioForEachFormat(t *testing.T) {
	p := audioPacket{frame: 7, opus: []byte{0xfc, 9}}
	// Legacy: header (Opus, context), session, frame, length, data.
	want := []byte{4<<5 | 2, 5, 7, 2, 0xfc, 9}
	legacy := encodeServerAudio(p, 5, ContextWhisper, true, true)
	if !bytes.Equal(legacy, want) {
		t.Fatalf("legacy %x, want %x", legacy, want)
	}
	if got, ok := decodeLegacyServerAudio(legacy); !ok || got.session != 5 || got.context != ContextWhisper || got.frame != 7 {
		t.Fatalf("legacy back %+v", got)
	}
	pb := encodeServerAudio(p, 5, ContextShout, false, true)
	if pb[0] != 0 {
		t.Fatalf("protobuf type byte %x", pb[0])
	}
	got, ok := decodeAudio(pb, false)
	if !ok || got.session != 5 || got.context != ContextShout || got.frame != 7 || !bytes.Equal(got.opus, p.opus) {
		t.Fatalf("protobuf %+v", got)
	}
}

func TestBrokenAudioIsRefused(t *testing.T) {
	for _, b := range [][]byte{nil, {4 << 5}, {4 << 5, 1, 0x7f}, {2 << 5, 1, 1}, {0, 0xff}} {
		if _, ok := decodeAudio(b, b != nil && len(b) > 0 && b[0] != 0); ok {
			t.Errorf("accepted %x", b)
		}
	}
}

// decodeLegacyServerAudio reads a legacy packet from the server: like a
// client's, with the speaker's session after the header byte.
func decodeLegacyServerAudio(b []byte) (audioPacket, bool) {
	if len(b) < 2 {
		return audioPacket{}, false
	}
	session, n, ok := readLegacyVarint(b[1:])
	if !ok {
		return audioPacket{}, false
	}
	p, ok := decodeLegacyAudio(append([]byte{b[0]}, b[1+n:]...))
	p.session, p.context, p.target = uint32(session), AudioContext(p.target), 0
	return p, ok
}

func TestLegacyPositionIsLittleEndian(t *testing.T) {
	// PacketDataStream copies the float's bytes in native (little-endian)
	// order.
	b := []byte{4 << 5, 1, 1, 0xfc, 0x00, 0x00, 0xc0, 0x3f, 0x00, 0x00, 0x00, 0xc0, 0x00, 0x00, 0x50, 0x40}
	p, ok := decodeAudio(b, true)
	if !ok || !slices.Equal(p.positional, []float32{1.5, -2, 3.25}) {
		t.Fatalf("positional %v ok %v", p.positional, ok)
	}
	if got := encodeServerAudio(p, 9, ContextNormal, true, true); !bytes.Equal(got[3:], b[2:]) {
		t.Fatalf("encoded %x", got)
	}
}

func TestAudioMumbleWouldDropIsRefused(t *testing.T) {
	big := audioPacket{frame: 1, opus: make([]byte, 0x2000)}
	short := audioPacket{frame: 1, opus: []byte{0xfc}, positional: []float32{1}}
	for name, c := range map[string]struct {
		b      []byte
		legacy bool
	}{
		"opus too long (protobuf)":   {encodeClientAudio(big, false), false},
		"empty opus":                 {encodeClientAudio(audioPacket{frame: 1}, false), false},
		"legacy trailing bytes":      {append(encodeClientAudio(audioPacket{frame: 1, opus: []byte{0xfc}}, true), 1, 2), true},
		"target with the wrong type": {[]byte{0, 0x0a, 0x01, 0x00}, false},
	} {
		if _, ok := decodeAudio(c.b, c.legacy); ok {
			t.Errorf("%s: accepted", name)
		}
	}
	if p, ok := decodeAudio(encodeClientAudio(short, false), false); !ok || p.positional != nil {
		t.Errorf("one float: %+v %v, want the packet without position", p, ok)
	}
}

// encodeClientAudio is a client's packet (the test client).
func encodeClientAudio(p audioPacket, legacy bool) []byte {
	if legacy {
		return appendLegacyBody([]byte{legacyOpus<<5 | byte(p.target)&0x1f}, p)
	}
	b := []byte{0}
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(p.target))
	return appendProtobufBody(b, p)
}
