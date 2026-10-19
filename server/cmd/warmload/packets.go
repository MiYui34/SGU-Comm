package main

import "errors"

// oggPackets returns the audio packets of an Ogg Opus file (without the
// two header packets).
func oggPackets(buf []byte) ([][]byte, error) {
	var packets [][]byte
	var cur []byte
	for pos := 0; pos+27 <= len(buf); {
		segs := int(buf[pos+26])
		data := pos + 27 + segs
		if data > len(buf) {
			return nil, errors.New("broken Ogg page")
		}
		for _, l := range buf[pos+27 : data] {
			if data+int(l) > len(buf) {
				return nil, errors.New("broken Ogg page")
			}
			cur = append(cur, buf[data:data+int(l)]...)
			data += int(l)
			if l < 255 {
				packets = append(packets, cur)
				cur = nil
			}
		}
		pos = data
	}
	if len(packets) < 3 {
		return nil, errors.New("no audio packets")
	}
	return packets[2:], nil
}

// voicePacket is frame n of one long spurt in the legacy Opus format (the
// server speaks 1.4 to us); frame numbers count 10 ms units.
func voicePacket(n int, opus []byte) []byte {
	p := append([]byte{4 << 5}, varint(uint64(n*2))...)
	p = append(p, varint(uint64(len(opus)))...)
	return append(p, opus...)
}

// varint: Mumble's legacy varint, enough for these values.
func varint(v uint64) []byte {
	switch {
	case v < 0x80:
		return []byte{byte(v)}
	case v < 0x4000:
		return []byte{byte(v>>8) | 0x80, byte(v)}
	case v < 0x200000:
		return []byte{byte(v>>16) | 0xc0, byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v>>24) | 0xe0, byte(v >> 16), byte(v >> 8), byte(v)}
	}
}
