package mumble

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// EncodeFrame makes a Mumble TCP frame.
func EncodeFrame(typ uint16, body []byte) []byte {
	out := make([]byte, headerSize, headerSize+len(body))
	binary.BigEndian.PutUint16(out, typ)
	binary.BigEndian.PutUint32(out[2:], uint32(len(body)))
	return append(out, body...)
}

// MessageFrame makes a Mumble TCP frame from a protobuf message.
func MessageFrame(typ uint16, msg proto.Message) []byte {
	body, _ := proto.Marshal(msg) // our own messages always marshal
	return EncodeFrame(typ, body)
}

// frameReader splits a Mumble TCP byte stream into frames (u16 type, u32
// length, body), across any chunk boundaries. Bodies of frames that are not
// kept are skipped, not copied: voice is most of the traffic.
type frameReader struct {
	header []byte
	typ    uint16
	body   []byte
	need   int // body bytes still to read (or skip) for the current frame
	keep   bool
}

func (f *frameReader) feed(chunk []byte, keep func(typ uint16) bool, handle func(typ uint16, body []byte)) error {
	for len(chunk) > 0 {
		// need == 0: between frames (empty bodies are handled at once).
		if f.need == 0 {
			n := min(headerSize-len(f.header), len(chunk))
			f.header = append(f.header, chunk[:n]...)
			chunk = chunk[n:]
			if len(f.header) < headerSize {
				break
			}
			f.typ = binary.BigEndian.Uint16(f.header)
			length := binary.BigEndian.Uint32(f.header[2:])
			f.header = f.header[:0]
			if length > MaxPayload {
				return fmt.Errorf("frame too large: %d bytes", length)
			}
			f.need = int(length)
			f.keep = keep(f.typ)
			f.body = f.body[:0]
			if f.need == 0 && f.keep {
				handle(f.typ, f.body)
			}
			continue
		}
		n := min(f.need, len(chunk))
		if f.keep {
			f.body = append(f.body, chunk[:n]...)
		}
		f.need -= n
		chunk = chunk[n:]
		if f.need == 0 && f.keep {
			handle(f.typ, f.body)
		}
	}
	return nil
}
