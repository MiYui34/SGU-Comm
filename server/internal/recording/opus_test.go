package recording

import "testing"

func TestPacketSamples(t *testing.T) {
	for _, c := range []struct {
		packet  []byte
		samples int
		stereo  bool
		ok      bool
	}{
		{[]byte{31 << 3, 1}, 960, false, true},             // CELT 20 ms
		{[]byte{31<<3 | 4, 1}, 960, true, true},            // stereo
		{[]byte{28 << 3}, 120, false, true},                // CELT 2.5 ms
		{[]byte{1 << 3, 1}, 960, false, true},              // SILK 20 ms
		{[]byte{3 << 3, 1}, 2880, false, true},             // SILK 60 ms
		{[]byte{12 << 3, 1}, 480, false, true},             // hybrid 10 ms
		{[]byte{31<<3 | 1, 1, 1}, 1920, false, true},       // two frames
		{[]byte{31<<3 | 3, 3, 1, 1, 1}, 2880, false, true}, // code 3: three frames
		{[]byte{31<<3 | 3, 7}, 0, false, false},            // 140 ms: too long
		{[]byte{31<<3 | 3}, 0, false, false},               // no frame count
		{[]byte{31<<3 | 3, 0}, 0, false, false},            // no frames
		{nil, 0, false, false},
	} {
		samples, stereo, ok := packetSamples(c.packet)
		if samples != c.samples || stereo != c.stereo || ok != c.ok {
			t.Errorf("%x: %d %v %v, want %d %v %v", c.packet, samples, stereo, ok, c.samples, c.stereo, c.ok)
		}
	}
}

func TestSilenceCoversTheGap(t *testing.T) {
	for _, n := range []int{0, 120, 960, 1080, 5000 * 120} {
		total := 0
		for _, p := range silence(n, true) {
			s, stereo, ok := packetSamples(p)
			if !ok || !stereo {
				t.Fatalf("silence packet %x", p)
			}
			total += s
		}
		if total != n {
			t.Errorf("silence for %d covers %d", n, total)
		}
	}
	if p := silence(960, false)[0]; p[0]&4 != 0 {
		t.Errorf("mono silence %x is stereo", p)
	}
}
