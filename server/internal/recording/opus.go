package recording

// Opus packets (RFC 6716, section 3.1): the TOC byte tells the mode and
// frame size (config, top five bits), stereo (bit 2) and the frame count
// (code, low two bits).

const (
	sampleRate   = 48000
	grid         = 120  // 2.5 ms: every Opus duration is a multiple
	maxPacket    = 5760 // 120 ms
	frameSamples = 480  // Mumble's sequence numbers count 10 ms frames
)

// frameSize: samples per frame (48 kHz) by config. SILK: 10, 20, 40, 60
// ms; hybrid: 10, 20 ms; CELT: 2.5, 5, 10, 20 ms.
func frameSize(config byte) int {
	switch {
	case config < 12:
		return []int{480, 960, 1920, 2880}[config%4]
	case config < 16:
		return []int{480, 960}[config%2]
	default:
		return []int{120, 240, 480, 960}[config%4]
	}
}

// packetSamples: a packet's duration in samples (48 kHz) and whether it
// is stereo.
func packetSamples(p []byte) (samples int, stereo bool, ok bool) {
	if len(p) == 0 {
		return 0, false, false
	}
	frames := 1
	switch p[0] & 3 {
	case 1, 2:
		frames = 2
	case 3:
		if len(p) < 2 {
			return 0, false, false
		}
		frames = int(p[1] & 0x3f)
	}
	samples = frames * frameSize(p[0]>>3)
	if samples == 0 || samples > maxPacket {
		return 0, false, false
	}
	return samples, p[0]&4 != 0, true
}

// silence: CELT packets that decode to silence (the silence flag is the
// first symbol, whatever the frame size), n samples (a multiple of grid)
// in 20 ms frames and shorter ones for the rest.
func silence(n int, stereo bool) [][]byte {
	var s byte
	if stereo {
		s = 4
	}
	var out [][]byte
	for config := byte(31); config >= 28; config-- {
		for size := frameSize(config); n >= size; n -= size {
			out = append(out, []byte{config<<3 | s, 0xff, 0xfe})
		}
	}
	return out
}
