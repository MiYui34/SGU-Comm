package mumbled

import (
	"encoding/binary"
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

// audioPacket is a voice packet in either UDP format: protobuf (Mumble
// 1.5 on) or legacy (before 1.5, PacketDataStream).
type audioPacket struct {
	target     uint32 // client to server: 0 channel, 1-30 voice target, 31 loopback
	context    AudioContext
	session    uint32 // server to client: the speaker
	frame      uint64
	opus       []byte
	terminator bool
	positional []float32
}

const (
	legacyOpus       = 4
	legacyTerminator = 0x2000
	legacyLength     = 0x1fff
)

// appendLegacyVarint writes Mumble's varint: the length is in the prefix
// bits of the first byte. Only non-negative values are needed.
func appendLegacyVarint(b []byte, v uint64) []byte {
	switch {
	case v < 0x80:
		return append(b, byte(v))
	case v < 0x4000:
		return append(b, 0x80|byte(v>>8), byte(v))
	case v < 0x200000:
		return append(b, 0xc0|byte(v>>16), byte(v>>8), byte(v))
	case v < 0x10000000:
		return append(b, 0xe0|byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	case v <= math.MaxUint32:
		return binary.BigEndian.AppendUint32(append(b, 0xf0), uint32(v))
	default:
		return binary.BigEndian.AppendUint64(append(b, 0xf4), v)
	}
}

// readLegacyVarint returns the value and the bytes read. Negative forms
// are refused (no voice field uses them).
func readLegacyVarint(b []byte) (uint64, int, bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	tail := func(first uint64, n int) (uint64, int, bool) {
		if len(b) < 1+n {
			return 0, 0, false
		}
		v := first
		for _, x := range b[1 : 1+n] {
			v = v<<8 | uint64(x)
		}
		return v, 1 + n, true
	}
	c := b[0]
	switch {
	case c&0x80 == 0:
		return uint64(c), 1, true
	case c&0xc0 == 0x80:
		return tail(uint64(c&0x3f), 1)
	case c&0xe0 == 0xc0:
		return tail(uint64(c&0x1f), 2)
	case c&0xf0 == 0xe0:
		return tail(uint64(c&0x0f), 3)
	case c&0xfc == 0xf0:
		return tail(0, 4)
	case c&0xfc == 0xf4:
		return tail(0, 8)
	}
	return 0, 0, false
}

// decodeAudio reads a client's voice packet (or, for tests, a server's).
func decodeAudio(b []byte, legacy bool) (audioPacket, bool) {
	if legacy {
		return decodeLegacyAudio(b)
	}
	return decodeProtobufAudio(b)
}

func decodeLegacyAudio(b []byte) (audioPacket, bool) {
	var p audioPacket
	if len(b) < 1 || b[0]>>5 != legacyOpus {
		return p, false
	}
	p.target = uint32(b[0] & 0x1f)
	frame, n, ok := readLegacyVarint(b[1:])
	if !ok {
		return p, false
	}
	rest := b[1+n:]
	header, n, ok := readLegacyVarint(rest)
	if !ok {
		return p, false
	}
	rest = rest[n:]
	length := int(header & legacyLength)
	if length == 0 || length > len(rest) {
		return p, false
	}
	p.frame, p.terminator = frame, header&legacyTerminator != 0
	p.opus, rest = rest[:length], rest[length:]
	switch len(rest) {
	case 0:
	case 12: // position: three floats in native (little-endian) order
		for i := range 3 {
			p.positional = append(p.positional, math.Float32frombits(binary.LittleEndian.Uint32(rest[4*i:])))
		}
	default:
		return p, false // as Mumble
	}
	return p, true
}

func decodeProtobufAudio(b []byte) (audioPacket, bool) {
	var p audioPacket
	if len(b) < 1 || b[0] != 0 {
		return p, false
	}
	b = b[1:]
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return p, false
		}
		b = b[n:]
		var v uint64
		var bytes []byte
		switch typ {
		case protowire.VarintType:
			v, n = protowire.ConsumeVarint(b)
		case protowire.BytesType:
			bytes, n = protowire.ConsumeBytes(b)
		case protowire.Fixed32Type: // an unpacked float
			var f uint32
			f, n = protowire.ConsumeFixed32(b)
			bytes = binary.LittleEndian.AppendUint32(nil, f)
		default:
			n = protowire.ConsumeFieldValue(num, typ, b)
		}
		if n < 0 || !audioWireType(num, typ) {
			return p, false
		}
		b = b[n:]
		switch num {
		case 1:
			if v > loopbackTarget {
				return p, false
			}
			p.target = uint32(v)
		case 2:
			p.context = AudioContext(v)
		case 3:
			p.session = uint32(v)
		case 4:
			p.frame = v
		case 5:
			p.opus = bytes
		case 6: // floats, packed or not
			for len(bytes) >= 4 {
				p.positional = append(p.positional, math.Float32frombits(binary.LittleEndian.Uint32(bytes)))
				bytes = bytes[4:]
			}
		case 16:
			p.terminator = v != 0
		}
	}
	// Mumble drops positions that are not three floats: keep three, or none.
	if len(p.positional) >= 3 {
		p.positional = p.positional[:3]
	} else {
		p.positional = nil
	}
	if len(p.opus) == 0 || len(p.opus) > legacyLength {
		return p, false // the legacy format could not carry it either
	}
	return p, true
}

// audioWireType: the fields we read must have their wire type.
func audioWireType(num protowire.Number, typ protowire.Type) bool {
	switch num {
	case 1, 2, 3, 4, 16:
		return typ == protowire.VarintType
	case 5:
		return typ == protowire.BytesType
	case 6:
		return typ == protowire.BytesType || typ == protowire.Fixed32Type
	}
	return true
}

// encodeServerAudio is a packet as one receiver gets it, in its format.
// The position goes only to receivers in the same game (plugin context).
func encodeServerAudio(p audioPacket, session uint32, ctx AudioContext, legacy, withPosition bool) []byte {
	if !withPosition {
		p.positional = nil
	}
	if legacy {
		b := make([]byte, 1, len(p.opus)+32)
		b[0] = legacyOpus<<5 | byte(ctx)&0x1f
		b = appendLegacyVarint(b, uint64(session))
		return appendLegacyBody(b, p)
	}
	b := make([]byte, 1, len(p.opus)+48)
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(ctx))
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(session))
	return appendProtobufBody(b, p)
}

func appendLegacyBody(b []byte, p audioPacket) []byte {
	b = appendLegacyVarint(b, p.frame)
	header := uint64(len(p.opus) & legacyLength)
	if p.terminator {
		header |= legacyTerminator
	}
	b = appendLegacyVarint(b, header)
	b = append(b, p.opus...)
	if len(p.positional) == 3 {
		for _, f := range p.positional {
			b = binary.LittleEndian.AppendUint32(b, math.Float32bits(f))
		}
	}
	return b
}

func appendProtobufBody(b []byte, p audioPacket) []byte {
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, p.frame)
	b = protowire.AppendTag(b, 5, protowire.BytesType)
	b = protowire.AppendBytes(b, p.opus)
	if len(p.positional) > 0 {
		var floats []byte
		for _, f := range p.positional {
			floats = binary.LittleEndian.AppendUint32(floats, math.Float32bits(f))
		}
		b = protowire.AppendTag(b, 6, protowire.BytesType)
		b = protowire.AppendBytes(b, floats)
	}
	if p.terminator {
		b = protowire.AppendTag(b, 16, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	return b
}
