package recording

import (
	"encoding/binary"
	"io"
)

// A small Ogg muxer (RFC 3533) for one logical stream. Packets never span
// pages: Mumble's voice packets are at most 8191 bytes (33 segments).

const (
	flagBOS = 2
	flagEOS = 4
	// A page goes out when its segments are full or it holds this much
	// audio, so a crash loses at most about a second.
	pageSamples = sampleRate
)

var crcTable = func() (t [256]uint32) {
	for i := range t {
		r := uint32(i) << 24
		for range 8 {
			if r&0x80000000 != 0 {
				r = r<<1 ^ 0x04c11db7
			} else {
				r <<= 1
			}
		}
		t[i] = r
	}
	return
}()

func oggCRC(b []byte) uint32 {
	var crc uint32
	for _, x := range b {
		crc = crc<<8 ^ crcTable[byte(crc>>24)^x]
	}
	return crc
}

type oggWriter struct {
	w      io.Writer
	serial uint32
	seq    uint32
	bytes  int64 // written so far
	// The page being filled: its packets, segments, and the granule
	// position at its last packet and before its first.
	pending  [][]byte
	segments int
	body     int // bytes of the pending packets
	granule  int64
	start    int64
}

// buildPage: one page with these complete packets.
func buildPage(packets [][]byte, granule int64, serial, seq uint32, flags byte) []byte {
	var lacing, body []byte
	for _, p := range packets {
		for n := len(p); ; n -= 255 {
			if n < 255 {
				lacing = append(lacing, byte(n))
				break
			}
			lacing = append(lacing, 255)
		}
		body = append(body, p...)
	}
	b := make([]byte, 27, 27+len(lacing)+len(body))
	copy(b, "OggS")
	b[5] = flags
	binary.LittleEndian.PutUint64(b[6:], uint64(granule))
	binary.LittleEndian.PutUint32(b[14:], serial)
	binary.LittleEndian.PutUint32(b[18:], seq)
	b[26] = byte(len(lacing))
	b = append(append(b, lacing...), body...)
	binary.LittleEndian.PutUint32(b[22:], oggCRC(b))
	return b
}

func segments(p []byte) int { return len(p)/255 + 1 }

// page writes packets as a page of their own (headers).
func (o *oggWriter) page(packets [][]byte, granule int64, flags byte) error {
	b := buildPage(packets, granule, o.serial, o.seq, flags)
	o.seq++
	n, err := o.w.Write(b)
	o.bytes += int64(n)
	return err
}

// packet adds an audio packet that ends at granule.
func (o *oggWriter) packet(p []byte, granule int64) error {
	if len(o.pending) > 0 && (o.segments+segments(p) > 255 || o.granule-o.start >= pageSamples) {
		if err := o.flush(0); err != nil {
			return err
		}
	}
	if len(o.pending) == 0 {
		o.start = o.granule
	}
	o.pending, o.segments, o.granule = append(o.pending, p), o.segments+segments(p), granule
	o.body += len(p)
	return nil
}

func (o *oggWriter) flush(flags byte) error {
	err := o.page(o.pending, o.granule, flags)
	o.pending, o.segments, o.body = nil, 0, 0
	return err
}

// size: the bytes written and those of the pending page.
func (o *oggWriter) size() int64 {
	if len(o.pending) == 0 {
		return o.bytes
	}
	return o.bytes + int64(27+o.segments+o.body)
}

// close writes the last page (end of stream).
func (o *oggWriter) close() error { return o.flush(flagEOS) }
