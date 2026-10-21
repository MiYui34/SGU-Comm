package mediasize

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"testing"

	"kallisti.chat/server/internal/imagesize"
)

func probe(data []byte, ctype string) Info {
	return Probe(bytes.NewReader(data), int64(len(data)), ctype)
}

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func be16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func be64(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

func encoded(t *testing.T, enc func(*bytes.Buffer, image.Image) error, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := enc(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngOf(t *testing.T, w, h int) []byte {
	return encoded(t, func(b *bytes.Buffer, m image.Image) error { return png.Encode(b, m) }, w, h)
}

func jpegOf(t *testing.T, w, h int) []byte {
	return encoded(t, func(b *bytes.Buffer, m image.Image) error { return jpeg.Encode(b, m, nil) }, w, h)
}

// withExif puts an APP1 segment with an orientation (big- or
// little-endian TIFF) after a JPEG's SOI marker.
func withExif(jpg []byte, orientation uint16, little bool) []byte {
	var o binary.AppendByteOrder = binary.BigEndian
	tiff := []byte("MM\x00\x2a")
	if little {
		o, tiff = binary.LittleEndian, []byte("II\x2a\x00")
	}
	tiff = o.AppendUint32(tiff, 8)      // IFD0 offset
	tiff = o.AppendUint16(tiff, 2)      // entries
	tiff = o.AppendUint16(tiff, 0x010f) // make: ignored
	tiff = o.AppendUint16(tiff, 2)
	tiff = o.AppendUint32(tiff, 1)
	tiff = o.AppendUint32(tiff, 0)
	tiff = o.AppendUint16(tiff, 0x0112) // orientation, SHORT, 1
	tiff = o.AppendUint16(tiff, 3)
	tiff = o.AppendUint32(tiff, 1)
	tiff = o.AppendUint16(tiff, orientation)
	tiff = o.AppendUint16(tiff, 0)
	tiff = o.AppendUint32(tiff, 0) // no next IFD
	app1 := append([]byte("Exif\x00\x00"), tiff...)
	seg := append([]byte{0xff, 0xe1}, be16(uint16(len(app1)+2))...)
	return append(append(append([]byte{}, jpg[:2]...), append(seg, app1...)...), jpg[2:]...)
}

// webpOf is a lossless WebP header (all DecodeConfig reads).
func webpOf(w, h int) []byte {
	bits := uint32(w-1) | uint32(h-1)<<14
	chunk := append([]byte{0x2f}, binary.LittleEndian.AppendUint32(nil, bits)...)
	b := append([]byte("VP8L"), binary.LittleEndian.AppendUint32(nil, uint32(len(chunk)))...)
	b = append(b, chunk...)
	return append(append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(len(b)+4))...), append([]byte("WEBP"), b...)...)
}

func TestImages(t *testing.T) {
	var g bytes.Buffer
	if err := gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 7, 5), color.Palette{color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, ctype string
		data        []byte
		want        Info
	}{
		{"png", "image/png", pngOf(t, 40, 30), Info{Width: 40, Height: 30}},
		{"jpeg", "image/jpeg", jpegOf(t, 40, 30), Info{Width: 40, Height: 30}},
		{"gif", "image/gif", g.Bytes(), Info{Width: 7, Height: 5}},
		{"webp", "image/webp", webpOf(30, 20), Info{Width: 30, Height: 20}},
		{"jpeg turned by exif", "image/jpeg", withExif(jpegOf(t, 40, 30), 6, false), Info{Width: 30, Height: 40}},
		{"jpeg turned by exif, little-endian", "image/jpeg", withExif(jpegOf(t, 40, 30), 8, true), Info{Width: 30, Height: 40}},
		{"jpeg upside down", "image/jpeg", withExif(jpegOf(t, 40, 30), 3, false), Info{Width: 40, Height: 30}},
		{"too large to show", "image/png", pngOf(t, 9000, 9000), Info{}},
		{"broken", "image/png", []byte("\x89PNG\r\n\x1a\nxx"), Info{}},
		{"not an image type", "application/octet-stream", pngOf(t, 4, 4), Info{}},
	} {
		if got := probe(c.data, c.ctype); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestExifOrientation(t *testing.T) {
	for _, c := range []struct {
		data []byte
		want uint16
	}{
		{jpegOf(t, 4, 3), 0},
		{withExif(jpegOf(t, 4, 3), 6, false), 6},
		{withExif(jpegOf(t, 4, 3), 3, true), 3},
		{pngOf(t, 4, 3), 0},
	} {
		if got := ExifOrientation(bytes.NewReader(c.data), int64(len(c.data))); got != c.want {
			t.Errorf("got %d, want %d", got, c.want)
		}
	}
}

func TestFixtures(t *testing.T) {
	for _, c := range []struct {
		name, ctype string
		want        Info
	}{
		{"moov-at-end.mp4", "video/mp4", Info{64, 48, 500}},
		{"faststart.mp4", "video/mp4", Info{64, 48, 500}},
		{"rotated.mp4", "video/mp4", Info{48, 64, 500}},
		{"audio.m4a", "audio/mp4", Info{0, 0, 700}},
		{"vp9.webm", "video/webm", Info{64, 48, 500}},
		{"aspect.webm", "video/webm", Info{85, 48, 500}},
		{"live.webm", "video/webm", Info{64, 48, 0}}, // no duration
	} {
		if got := probe(fixture(t, c.name), c.ctype); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func box(typ string, parts ...[]byte) []byte {
	body := bytes.Join(parts, nil)
	return append(append(be32(uint32(8+len(body))), typ...), body...)
}

func mvhd(version byte, timescale uint32, duration uint64) []byte {
	if version == 1 {
		return box("mvhd", []byte{1, 0, 0, 0}, be64(0), be64(0), be32(timescale), be64(duration), make([]byte, 80))
	}
	return box("mvhd", []byte{0, 0, 0, 0}, be32(0), be32(0), be32(timescale), be32(uint32(duration)), make([]byte, 80))
}

var (
	identity = [9]int32{1 << 16, 0, 0, 0, 1 << 16, 0, 0, 0, 1 << 30}
	turned90 = [9]int32{0, 1 << 16, 0, -1 << 16, 0, 0, 0, 0, 1 << 30}
	turned27 = [9]int32{0, -1 << 16, 0, 1 << 16, 0, 0, 0, 0, 1 << 30}
	turned18 = [9]int32{-1 << 16, 0, 0, 0, -1 << 16, 0, 0, 0, 1 << 30}
)

func tkhd(version byte, w, h float64, m [9]int32) []byte {
	head := [][]byte{{version, 0, 0, 3}, be32(0), be32(0), be32(1), be32(0), be32(0)}
	if version == 1 {
		head = [][]byte{{1, 0, 0, 3}, be64(0), be64(0), be32(1), be32(0), be64(0)}
	}
	var matrix []byte
	for _, v := range m {
		matrix = append(matrix, be32(uint32(v))...)
	}
	return box("tkhd", append(head, make([]byte, 16), matrix, be32(uint32(w*65536)), be32(uint32(h*65536)))...)
}

func mp4(moov ...[]byte) []byte {
	return append(box("ftyp", []byte("isom"), be32(0)), box("moov", moov...)...)
}

func TestMP4(t *testing.T) {
	audio := box("trak", tkhd(0, 0, 0, identity))
	video := func(m [9]int32) []byte { return box("trak", tkhd(0, 1280, 720, m)) }
	large := append(append(be32(1), "free"...), be64(16+4)...)
	large = append(large, "pad!"...)
	for _, c := range []struct {
		name string
		data []byte
		want Info
	}{
		{"plain", mp4(mvhd(0, 1000, 2500), video(identity)), Info{1280, 720, 2500}},
		{"version 1", mp4(mvhd(1, 90000, 90000*3), box("trak", tkhd(1, 640.5, 360, identity))), Info{640, 360, 3000}},
		{"audio track first", mp4(mvhd(0, 1000, 10), audio, video(identity)), Info{1280, 720, 10}},
		{"turned 90", mp4(mvhd(0, 1000, 10), video(turned90)), Info{720, 1280, 10}},
		{"turned 270", mp4(mvhd(0, 1000, 10), video(turned27)), Info{720, 1280, 10}},
		{"turned 180", mp4(mvhd(0, 1000, 10), video(turned18)), Info{1280, 720, 10}},
		{"64-bit box size before moov", append(append(box("ftyp", []byte("isom")), large...), box("moov", video(identity))...), Info{1280, 720, 0}},
		{"box to the end of the file", append(box("ftyp", []byte("isom")), append(be32(0), append([]byte("moov"), video(identity)...)...)...), Info{1280, 720, 0}},
		{"unknown duration", mp4(mvhd(0, 1000, math.MaxUint32), video(identity)), Info{1280, 720, 0}},
		{"no timescale", mp4(mvhd(0, 0, 10), video(identity)), Info{1280, 720, 0}},
		{"huge duration", mp4(mvhd(1, 1, math.MaxUint64-1), video(identity)), Info{1280, 720, 0}},
		{"unknown duration, version 1", mp4(mvhd(1, 90000, math.MaxUint64), video(identity)), Info{1280, 720, 0}},
		{"no moov", box("ftyp", []byte("isom")), Info{}},
	} {
		if got := probe(c.data, "video/mp4"); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

// avif is a hand-made AVIF header: ftyp with the brands, then meta with
// the given property boxes (ispe, irot) in iprp/ipco.
func avif(brands string, props ...[]byte) []byte {
	ftyp := box("ftyp", []byte(brands[:4]), be32(0), []byte(brands))
	return append(ftyp, box("meta", []byte{0, 0, 0, 0}, box("hdlr", make([]byte, 24)), box("iprp", box("ipco", props...)))...)
}

func ispe(w, h uint32) []byte { return box("ispe", []byte{0, 0, 0, 0}, be32(w), be32(h)) }
func irot(angle byte) []byte  { return box("irot", []byte{angle}) }

func TestAVIF(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want Info
	}{
		{"still (ffmpeg)", fixture(t, "still.avif"), Info{64, 48, 0}},
		{"grid of 2x2 tiles (avifenc)", fixture(t, "grid.avif"), Info{128, 96, 0}},
		{"turned by 90 degrees (avifenc --irot 1)", fixture(t, "rotated.avif"), Info{96, 128, 0}},
		{"animated (ffmpeg)", fixture(t, "anim.avif"), Info{48, 32, 0}},
		{"hand-made", avif("avifmif1", ispe(640, 480)), Info{640, 480, 0}},
		{"grid: the largest", avif("mif1avif", ispe(512, 512), ispe(1000, 700), ispe(512, 512)), Info{1000, 700, 0}},
		{"turned 270", avif("avif", ispe(640, 480), irot(3)), Info{480, 640, 0}},
		{"turned 180", avif("avif", ispe(640, 480), irot(2)), Info{640, 480, 0}},
		{"sequence without meta: its track", append(box("ftyp", []byte("avis"), be32(0), []byte("avis")), box("moov", mvhd(0, 1000, 500), box("trak", tkhd(0, 320, 240, identity)))...), Info{320, 240, 0}},
		{"too wide to show", avif("avif", ispe(imagesize.MaxSide+1, 1)), Info{}},
		{"too many pixels to show", avif("avif", ispe(9000, 9000)), Info{}},
		{"zero height", avif("avif", ispe(640, 0)), Info{}},
		{"no ispe", avif("avif"), Info{}},
		{"ispe too short", avif("avif", box("ispe", []byte{0, 0, 0, 0}, be32(640))), Info{}},
	} {
		if got := probe(c.data, "image/avif"); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestDetectAVIF(t *testing.T) {
	for _, c := range []struct {
		name string
		head []byte
		want bool
	}{
		{"still", fixture(t, "still.avif"), true},
		{"animated", fixture(t, "anim.avif"), true},
		{"compatible brand only", box("ftyp", []byte("mif1"), be32(0), []byte("mif1miafavif")), true},
		{"major brand only", box("ftyp", []byte("avis"), be32(0)), true},
		{"mp4", fixture(t, "faststart.mp4"), false},
		{"HEIC", box("ftyp", []byte("heic"), be32(0), []byte("mif1heic")), false},
		{"brand in the minor version", box("ftyp", []byte("mif1"), []byte("avif")), false},
		{"brand after the ftyp box", append(box("ftyp", []byte("mif1"), be32(0)), "avif"...), false},
		{"ftyp not first", append(box("free"), box("ftyp", []byte("avif"), be32(0))...), false},
		{"ftyp too short", append(be32(12), "ftypavif"...), false},
		{"ftyp cut", append(be32(0x20), "ftypmif1\x00\x00\x00\x00av"...), false},
		{"cut after a brand", append(be32(0x20), "ftypmif1\x00\x00\x00\x00avif"...), true},
		{"empty", nil, false},
	} {
		if got := DetectType(c.head) == "image/avif"; got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// ebml is an element with an 8-byte size.
func ebml(id uint32, parts ...[]byte) []byte {
	idb := be32(id)
	for len(idb) > 1 && idb[0] == 0 {
		idb = idb[1:]
	}
	body := bytes.Join(parts, nil)
	return append(append(idb, 0x01), append(be64(uint64(len(body)))[1:], body...)...)
}

func ebmlUint(id uint32, v uint64) []byte { return ebml(id, be64(v)[8-max(1, (bitsLen(v)+7)/8):]) }

func bitsLen(v uint64) int {
	n := 0
	for ; v > 0; v >>= 8 {
		n += 8
	}
	return n
}

func ebmlFloat32(id uint32, f float32) []byte { return ebml(id, be32(math.Float32bits(f))) }
func ebmlFloat64(id uint32, f float64) []byte { return ebml(id, be64(math.Float64bits(f))) }

// unknownSize is an element of unknown size (to the end of its parent).
func unknownSize(id uint32, parts ...[]byte) []byte {
	return append(append(be32(id), 0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff), bytes.Join(parts, nil)...)
}

func webm(segment ...[]byte) []byte {
	return append(ebml(idEBML, ebml(0x4282, []byte("webm"))), ebml(idSegment, segment...)...)
}

func videoTrack(video ...[]byte) []byte {
	return ebml(idTracks, ebml(0xae, ebmlUint(0x83, 1), ebml(0xe0, video...)))
}

func TestWebM(t *testing.T) {
	pixels := [][]byte{ebmlUint(0xb0, 640), ebmlUint(0xba, 480)}
	audioTrack := ebml(0xae, ebmlUint(0x83, 2))
	for _, c := range []struct {
		name string
		data []byte
		want Info
	}{
		{"plain", webm(ebml(idInfo, ebmlFloat64(0x4489, 1500)), videoTrack(pixels...)), Info{640, 480, 1500}},
		{"float32 duration, timecode scale", webm(ebml(idInfo, ebmlUint(0x2ad7b1, 1000), ebmlFloat32(0x4489, 2_000_000)), videoTrack(pixels...)), Info{640, 480, 2000}},
		{"display size", webm(videoTrack(append(pixels, ebmlUint(0x54b0, 853), ebmlUint(0x54ba, 480))...)), Info{853, 480, 0}},
		{"display aspect ratio", webm(videoTrack(append(pixels, ebmlUint(0x54b0, 16), ebmlUint(0x54ba, 9), ebmlUint(0x54b2, 3))...)), Info{853, 480, 0}},
		{"display unit cm: pixels", webm(videoTrack(append(pixels, ebmlUint(0x54b0, 3), ebmlUint(0x54ba, 2), ebmlUint(0x54b2, 1))...)), Info{640, 480, 0}},
		{"audio track first", webm(ebml(idTracks, audioTrack, ebml(0xae, ebmlUint(0x83, 1), ebml(0xe0, pixels...)))), Info{640, 480, 0}},
		{"audio only", webm(ebml(idInfo, ebmlFloat64(0x4489, 700)), ebml(idTracks, audioTrack)), Info{0, 0, 700}},
		{"unknown sizes (recorder)", append(ebml(idEBML), unknownSize(idSegment, ebml(idInfo, ebmlFloat64(0x4489, 10)), videoTrack(pixels...), unknownSize(idCluster, make([]byte, 64)))...), Info{640, 480, 10}},
		{"tracks after a cluster are not looked for", webm(ebml(idCluster, make([]byte, 8)), videoTrack(pixels...)), Info{}},
		{"empty timecode scale: the default", webm(ebml(idInfo, ebml(0x2ad7b1), ebmlFloat64(0x4489, 1500)), videoTrack(pixels...)), Info{640, 480, 1500}},
		{"negative duration", webm(ebml(idInfo, ebmlFloat64(0x4489, -5)), videoTrack(pixels...)), Info{640, 480, 0}},
		{"NaN duration", webm(ebml(idInfo, ebmlFloat64(0x4489, math.NaN())), videoTrack(pixels...)), Info{640, 480, 0}},
		{"huge duration", webm(ebml(idInfo, ebmlUint(0x2ad7b1, math.MaxUint64), ebmlFloat64(0x4489, 1e300)), videoTrack(pixels...)), Info{640, 480, 0}},
		{"not EBML", []byte("RIFF....WEBPVP8 "), Info{}},
	} {
		if got := probe(c.data, "video/webm"); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

// Every cut of a valid file, and hostile sizes: no panic, no endless loop,
// nothing but zeros or the full answer.
func TestMalformed(t *testing.T) {
	samples := map[string][]byte{
		"video/mp4":  fixture(t, "rotated.mp4"),
		"audio/mp4":  fixture(t, "audio.m4a"),
		"video/webm": fixture(t, "vp9.webm"),
		"image/jpeg": withExif(jpegOf(t, 40, 30), 6, true),
		"image/avif": fixture(t, "grid.avif"),
	}
	for ctype, data := range samples {
		full := probe(data, ctype)
		for n := range len(data) {
			got := probe(data[:n], ctype)
			if got.Width < 0 || got.Height < 0 || got.Duration < 0 || got.Width > full.Width || got.Height > full.Height+full.Width {
				t.Fatalf("%s cut at %d: %+v", ctype, n, got)
			}
		}
	}
	for _, c := range []struct {
		name, ctype string
		data        []byte
	}{
		{"box smaller than its header", "video/mp4", append(be32(4), "moov"...)},
		{"box larger than the file", "video/mp4", append(be32(1<<31), "moov"...)},
		{"64-bit size below 16", "video/mp4", append(append(be32(1), "moov"...), be64(8)...)},
		{"64-bit size overflowing", "video/mp4", append(append(be32(1), "moov"...), be64(math.MaxUint64)...)},
		{"many empty boxes", "video/mp4", bytes.Repeat(box("free"), 100000)},
		{"zero-size box inside moov", "video/mp4", box("moov", be32(0), []byte("trak"))},
		{"tkhd too short", "video/mp4", mp4(box("trak", box("tkhd", []byte{0, 0, 0, 0})))},
		{"mvhd too short", "video/mp4", mp4(box("mvhd", []byte{1, 0, 0, 0}))},
		{"bad EBML id", "video/webm", []byte{0x00, 0x00, 0x00, 0x00, 0x81}},
		{"bad EBML size", "video/webm", append(be32(idEBML), 0x00)},
		{"EBML size beyond the file", "video/webm", append(be32(idEBML), 0x01, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)},
		{"many empty elements", "video/webm", append(ebml(idEBML), unknownSize(idSegment, bytes.Repeat([]byte{0xec, 0x80}, 100000))...)},
		{"unknown-size info", "video/webm", append(ebml(idEBML), unknownSize(idSegment, unknownSize(idInfo, ebmlFloat64(0x4489, 1)))...)},
		{"pixel width of 9 bytes", "video/webm", webm(videoTrack(ebml(0xb0, make([]byte, 9)), ebmlUint(0xba, 1)))},
		{"zero display height", "video/webm", webm(videoTrack(ebmlUint(0xb0, 4), ebmlUint(0xba, 4), ebmlUint(0x54b0, 4), ebmlUint(0x54ba, 0), ebmlUint(0x54b2, 3)))},
		{"many empty boxes in ipco", "image/avif", avif("avif", bytes.Repeat(box("free"), 100000))},
		{"meta too short for its version", "image/avif", append(box("ftyp", []byte("avif"), be32(0)), box("meta")...)},
		{"ispe larger than ipco", "image/avif", append(box("ftyp", []byte("avif"), be32(0)), box("meta", be32(0), box("iprp", append(be32(8), "ipco"...), ispe(4, 4)))...)},
		{"exif offset beyond the segment", "image/jpeg", func() []byte {
			b := withExif(jpegOf(t, 4, 4), 6, false)
			b[2+2+2+6+4+3] = 0xff // IFD0 offset
			return b
		}()},
	} {
		got := probe(c.data, c.ctype)
		if got.Width < 0 || got.Height < 0 || got.Duration < 0 {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
}

func FuzzMP4(f *testing.F) {
	f.Add(fixture(f, "rotated.mp4"))
	f.Add(mp4(mvhd(1, 1000, 10), box("trak", tkhd(1, 10, 10, turned90))))
	f.Fuzz(func(t *testing.T, data []byte) {
		if got := probe(data, "video/mp4"); got.Width < 0 || got.Height < 0 || got.Duration < 0 {
			t.Fatal(got)
		}
	})
}

func FuzzAVIF(f *testing.F) {
	f.Add(fixture(f, "grid.avif"))
	f.Add(fixture(f, "anim.avif"))
	f.Fuzz(func(t *testing.T, data []byte) {
		DetectType(data)
		if got := probe(data, "image/avif"); got.Width < 0 || got.Height < 0 || got.Duration != 0 {
			t.Fatal(got)
		}
	})
}

func FuzzJPEG(f *testing.F) {
	f.Add(withExif([]byte("\xff\xd8\xff\xd9"), 6, false))
	f.Add(withExif([]byte("\xff\xd8\xff\xd9"), 8, true))
	f.Fuzz(func(t *testing.T, data []byte) {
		ExifOrientation(bytes.NewReader(data), int64(len(data)))
	})
}

func FuzzWebM(f *testing.F) {
	f.Add(fixture(f, "vp9.webm"))
	f.Add(fixture(f, "live.webm"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if got := probe(data, "video/webm"); got.Width < 0 || got.Height < 0 || got.Duration < 0 {
			t.Fatal(got)
		}
	})
}
