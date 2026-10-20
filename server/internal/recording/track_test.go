package recording

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

var (
	speech20  = []byte{1 << 3, 0xaa, 0xbb}    // SILK 20 ms, mono
	stereo20  = []byte{31<<3 | 4, 0xcc, 0xdd} // CELT 20 ms, stereo
	speech60  = []byte{3 << 3, 0xee}          // SILK 60 ms
	isSilence = func(p []byte) bool { return len(p) == 3 && p[1] == 0xff && p[2] == 0xfe }
)

// parsed is a track file: its header fields and audio packets with the
// sample where each starts.
type parsed struct {
	channels byte
	preSkip  uint16
	tags     []byte
	packets  [][]byte
	starts   []int64
	end      int64 // the last granule position
}

func parseTrack(t *testing.T, path string) parsed {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pages := readOgg(t, b)
	if len(pages) < 3 || len(pages[0].packets) != 1 || len(pages[1].packets) != 1 || pages[0].flags != flagBOS || pages[0].granule != 0 || pages[1].granule != 0 {
		t.Fatalf("header pages %+v", pages[:min(2, len(pages))])
	}
	head := pages[0].packets[0]
	if len(head) != 19 || string(head[:8]) != "OpusHead" || head[8] != 1 || binary.LittleEndian.Uint32(head[12:]) != sampleRate || head[18] != 0 {
		t.Fatalf("OpusHead %x", head)
	}
	tags := pages[1].packets[0]
	if string(tags[:8]) != "OpusTags" {
		t.Fatalf("OpusTags %q", tags)
	}
	out := parsed{channels: head[9], preSkip: binary.LittleEndian.Uint16(head[10:]), tags: tags}
	var pos int64
	for _, p := range pages[2:] {
		for _, packet := range p.packets {
			n, _, ok := packetSamples(packet)
			if !ok {
				t.Fatalf("bad packet %x", packet)
			}
			out.packets, out.starts = append(out.packets, packet), append(out.starts, pos)
			pos += int64(n)
		}
		if p.granule != pos {
			t.Fatalf("page %d: granule %d, packets end at %d", p.seq, p.granule, pos)
		}
	}
	if pages[len(pages)-1].flags != flagEOS {
		t.Fatal("no end of stream")
	}
	out.end = pos
	return out
}

// startOf: where the first packet equal to p starts (-1: none).
func (p parsed) startOf(packet []byte, from int) int64 {
	for i := from; i < len(p.packets); i++ {
		if bytes.Equal(p.packets[i], packet) {
			return p.starts[i]
		}
	}
	return -1
}

func newTestTrack(t *testing.T) (*track, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.opus")
	tr, err := createTrack(path, "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	return tr, path
}

const (
	ms      = sampleRate / 1000
	noLimit = 1 << 40 // bytes of room for silence
)

// Silence that would take more than the room writes nothing; the track
// goes on as before.
func TestSilenceOverTheRoomWritesNothing(t *testing.T) {
	tr, _ := newTestTrack(t)
	before := tr.bytes()
	if err := tr.add(speech20, 0, false, 3600*sampleRate, 10*padBytes); err != errSizeLimit {
		t.Fatalf("an hour of silence in %d bytes: %v", 10*padBytes, err)
	}
	if tr.bytes() != before || tr.pos != 0 || tr.talking || tr.first != -1 {
		t.Fatalf("written: %d bytes, pos %d", tr.bytes()-before, tr.pos)
	}
	if err := tr.add(speech20, 0, false, 5*sampleRate, 10*padBytes); err != nil || tr.first != 5*sampleRate {
		t.Fatalf("5 s of silence in %d bytes: %v, first %d", 10*padBytes, err, tr.first)
	}
}

func TestATrackStartsWithSilenceUntilTheFirstWords(t *testing.T) {
	tr, path := newTestTrack(t)
	tr.add(speech20, 100, false, 1500*ms, noLimit)
	tr.add(speech20, 102, true, 1520*ms, noLimit)
	if err := tr.finish(3000 * ms); err != nil {
		t.Fatal(err)
	}
	p := parseTrack(t, path)
	if p.startOf(speech20, 0) != 1500*ms || !isSilence(p.packets[0]) {
		t.Fatalf("speech starts at %d", p.startOf(speech20, 0))
	}
	if p.end != 3000*ms || p.channels != 1 || p.preSkip != 312 || !bytes.Contains(p.tags, []byte("TITLE=alice")) {
		t.Fatalf("end %d channels %d pre-skip %d tags %q", p.end, p.channels, p.preSkip, p.tags)
	}
	if tr.first != 1500*ms {
		t.Fatalf("first words at %d", tr.first)
	}
}

// Within a burst the sequence numbers place packets (jitter does not
// move them; lost ones become silence); a new burst (after a terminator
// or a long pause) starts where the clock says.
func TestPacketsLineUpBySequenceAndClock(t *testing.T) {
	tr, path := newTestTrack(t)
	tr.add(speech20, 0, false, 0, noLimit)
	tr.add(speech60, 2, false, 90*ms, noLimit)   // 70 ms late
	tr.add(speech20, 10, false, 120*ms, noLimit) // 8 and 9 lost: 20 ms gap
	tr.add(speech20, 10, false, 125*ms, noLimit) // a repeat: dropped
	tr.add(stereo20, 12, true, 150*ms, noLimit)
	tr.add(speech20, 500, false, 1000*ms, noLimit)  // new burst after the terminator
	tr.add(speech20, 9000, false, 3000*ms, noLimit) // a long pause without terminator
	tr.finish(3020 * ms)
	p := parseTrack(t, path)
	want := []struct {
		packet []byte
		at     int64
	}{{speech20, 0}, {speech60, 20 * ms}, {speech20, 100 * ms}, {stereo20, 120 * ms}, {speech20, 1000 * ms}, {speech20, 3000 * ms}}
	from := 0
	for _, w := range want {
		i := from
		for i < len(p.packets) && isSilence(p.packets[i]) {
			i++
		}
		if i == len(p.packets) || !bytes.Equal(p.packets[i], w.packet) || p.starts[i] != w.at {
			t.Fatalf("want %x at %d ms; packets %x starts %v", w.packet, w.at/ms, p.packets[from:], p.starts[from:])
		}
		from = i + 1
	}
	if from != len(p.packets) || p.end != 3020*ms {
		t.Fatalf("extra packets %x, end %d", p.packets[from:], p.end)
	}
	// The stereo packet made the whole track stereo.
	if p.channels != 2 {
		t.Fatalf("%d channels", p.channels)
	}
}

// A burst that starts while the last one still plays goes after it.
func TestOverlappingPacketsMoveLater(t *testing.T) {
	tr, path := newTestTrack(t)
	tr.add(speech60, 0, true, 0, noLimit)
	tr.add(speech20, 50, false, 30*ms, noLimit)
	tr.finish(0)
	p := parseTrack(t, path)
	if p.startOf(speech20, 0) != 60*ms || p.end != 80*ms {
		t.Fatalf("starts %v end %d", p.starts, p.end)
	}
}

func TestBadPacketsAreDropped(t *testing.T) {
	tr, path := newTestTrack(t)
	tr.add([]byte{31<<3 | 3}, 0, false, 0, noLimit)
	tr.add(speech20, 2, false, 20*ms, noLimit)
	tr.finish(0)
	if p := parseTrack(t, path); len(p.packets) != 2 || !isSilence(p.packets[0]) {
		t.Fatalf("packets %x", p.packets)
	}
}

// A client that starts counting anew (a new audio context) after a lost
// terminator: not repeats, but a new burst.
func TestLowerSequenceNumbersLaterAreANewBurst(t *testing.T) {
	tr, path := newTestTrack(t)
	tr.add(speech20, 100, false, 0, noLimit)
	tr.add(speech60, 60, false, 2000*ms, noLimit)
	tr.finish(0)
	if p := parseTrack(t, path); p.startOf(speech60, 0) != 2000*ms {
		t.Fatalf("starts %v", p.starts)
	}
}
