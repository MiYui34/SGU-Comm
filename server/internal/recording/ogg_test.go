package recording

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// page is a parsed Ogg page.
type page struct {
	flags   byte
	granule int64
	serial  uint32
	seq     uint32
	packets [][]byte // complete packets ending on this page
}

// readOgg parses a stream and checks every page's structure and CRC.
func readOgg(t *testing.T, b []byte) []page {
	t.Helper()
	var pages []page
	for len(b) > 0 {
		if len(b) < 27 || string(b[:4]) != "OggS" || b[4] != 0 {
			t.Fatalf("page %d: bad header", len(pages))
		}
		n := int(b[26])
		size := 27 + n
		for _, l := range b[27 : 27+n] {
			size += int(l)
		}
		raw := bytes.Clone(b[:size])
		crc := binary.LittleEndian.Uint32(raw[22:])
		binary.LittleEndian.PutUint32(raw[22:], 0)
		if got := slowCRC(raw); got != crc {
			t.Fatalf("page %d: CRC %08x, want %08x", len(pages), crc, got)
		}
		p := page{flags: b[5], granule: int64(binary.LittleEndian.Uint64(b[6:])), serial: binary.LittleEndian.Uint32(b[14:]), seq: binary.LittleEndian.Uint32(b[18:])}
		data, cur := b[27+n:size], []byte{}
		for _, l := range b[27 : 27+n] {
			cur, data = append(cur, data[:l]...), data[l:]
			if l < 255 {
				p.packets, cur = append(p.packets, cur), []byte{}
			}
		}
		if len(cur) > 0 {
			t.Fatalf("page %d: a packet continues on the next page", len(pages))
		}
		if p.seq != uint32(len(pages)) {
			t.Fatalf("page %d has number %d", len(pages), p.seq)
		}
		pages, b = append(pages, p), b[size:]
	}
	return pages
}

// slowCRC is the Ogg CRC bit by bit (polynomial 0x04c11db7, no
// reflection, start 0), to check the table version.
func slowCRC(b []byte) uint32 {
	var crc uint32
	for _, x := range b {
		crc ^= uint32(x) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func TestOggCRC(t *testing.T) {
	b := []byte("OggS and some more bytes")
	if got, want := oggCRC(b), slowCRC(b); got != want || got == 0 {
		t.Fatalf("crc %08x, want %08x", got, want)
	}
}

func TestOggPagesCarryPacketsAndGranules(t *testing.T) {
	var buf bytes.Buffer
	w := &oggWriter{w: &buf, serial: 7}
	if err := w.page([][]byte{[]byte("head")}, 0, flagBOS); err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte{1}, 510) // two full segments and an empty one
	for i := range 300 {
		p := []byte{byte(i)}
		if i == 5 {
			p = big
		}
		if err := w.packet(p, int64(i+1)*960); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.close(); err != nil {
		t.Fatal(err)
	}
	pages := readOgg(t, buf.Bytes())
	if pages[0].flags != flagBOS || string(pages[0].packets[0]) != "head" {
		t.Fatalf("first page %+v", pages[0])
	}
	last := pages[len(pages)-1]
	if last.flags != flagEOS || last.granule != 300*960 {
		t.Fatalf("last page flags %d granule %d", last.flags, last.granule)
	}
	count, prev := 0, int64(0)
	for _, p := range pages[1:] {
		count += len(p.packets)
		if p.serial != 7 || p.granule <= prev || p.granule != int64(count)*960 {
			t.Fatalf("page %d: serial %d granule %d after %d packets", p.seq, p.serial, p.granule, count)
		}
		prev = p.granule
	}
	if count != 300 || !bytes.Equal(pages[1].packets[5], big) {
		t.Fatalf("%d packets", count)
	}
	if w.bytes != int64(buf.Len()) {
		t.Fatalf("counted %d bytes, wrote %d", w.bytes, buf.Len())
	}
}
