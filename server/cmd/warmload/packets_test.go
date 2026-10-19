package main

import (
	"bytes"
	"testing"
)

// page builds an Ogg page around the packets (only what oggPackets reads:
// the segment count at 26 and the lacing table).
func page(packets ...[]byte) []byte {
	var table, data []byte
	for _, p := range packets {
		n := len(p)
		for ; n >= 255; n -= 255 {
			table = append(table, 255)
		}
		table = append(table, byte(n))
		data = append(data, p...)
	}
	h := make([]byte, 27)
	copy(h, "OggS")
	h[26] = byte(len(table))
	return append(append(h, table...), data...)
}

func TestOggPacketsSkipsHeadersAndJoinsSegments(t *testing.T) {
	long := bytes.Repeat([]byte{7}, 300)
	raw := append(page([]byte("OpusHead"), []byte("OpusTags")), page(long, []byte{1, 2})...)
	got, err := oggPackets(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !bytes.Equal(got[0], long) || !bytes.Equal(got[1], []byte{1, 2}) {
		t.Fatalf("got %d packets: %v", len(got), got)
	}
}

func TestOggPacketsNeedsAudio(t *testing.T) {
	if _, err := oggPackets(page([]byte("OpusHead"), []byte("OpusTags"))); err == nil {
		t.Fatal("no error without audio packets")
	}
	if _, err := oggPackets([]byte("OggS truncated page header garbage")); err == nil {
		t.Fatal("no error for a broken file")
	}
}

func TestVoicePacket(t *testing.T) {
	// Opus (4 << 5), frame number 200 as a two-byte varint, length 2.
	got := voicePacket(100, []byte{9, 9})
	want := []byte{4 << 5, 0x80, 200, 2, 9, 9}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
