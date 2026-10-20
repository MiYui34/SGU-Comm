// Package mediasize reads the size of pictures and videos and the length
// of videos and audio from their headers, so clients can keep the space
// for them before they load. Best effort: zero is unknown. Only headers
// are read, a bounded number of bytes whatever the input.
package mediasize

import (
	"cmp"
	"encoding/binary"
	"io"
	"math"
	"math/bits"

	"kallisti.chat/server/internal/imagesize"
)

type Info struct {
	Width, Height int
	Duration      int64 // milliseconds
}

const (
	// Larger sides are nonsense (or hostile): unknown.
	maxSide = 1<<16 - 1
	// Longer is nonsense too (or "unknown" in a field without a marker).
	maxDuration = 1e12 // milliseconds: 31 years
	// Box and element headers read per file at most.
	maxHeaders = 4096
)

// Probe reads what it can from a file of a type the server stores
// (see files.storedType).
func Probe(r io.ReaderAt, size int64, ctype string) Info {
	var i Info
	switch ctype {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		i = picture(r, size, ctype == "image/jpeg")
	case "video/mp4", "audio/mp4":
		i = (&walker{r: r, budget: maxHeaders}).mp4(size)
	case "video/webm":
		i = (&walker{r: r, budget: maxHeaders}).webm(size)
	}
	if i.Width <= 0 || i.Height <= 0 || i.Width > maxSide || i.Height > maxSide {
		i.Width, i.Height = 0, 0
	}
	if i.Duration < 0 || i.Duration > maxDuration {
		i.Duration = 0
	}
	return i
}

func picture(r io.ReaderAt, size int64, jpeg bool) Info {
	w, h, err := imagesize.Size(io.NewSectionReader(r, 0, size), imagesize.MaxSide, imagesize.MaxPixels)
	if err != nil {
		return Info{}
	}
	if jpeg && exifTurned(r, size) {
		w, h = h, w
	}
	return Info{Width: w, Height: h}
}

// exifTurned: a JPEG's Exif orientation turns it by 90 or 270 degrees
// (5 to 8), as browsers show it.
func exifTurned(r io.ReaderAt, size int64) bool {
	off := int64(2) // after SOI
	var h [4]byte
	for range 64 {
		if _, err := r.ReadAt(h[:], off); err != nil || h[0] != 0xff {
			return false
		}
		marker, n := h[1], int64(binary.BigEndian.Uint16(h[2:]))
		switch {
		case marker == 0xda || marker == 0xd9 || n < 2: // image data: no Exif before it
			return false
		case marker == 0xe1:
			seg := read(r, off+4, min(off+2+n, size), 1<<16)
			if len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
				o := orientation(seg[6:])
				return o >= 5 && o <= 8
			}
		}
		off += 2 + n
	}
	return false
}

// orientation reads tag 0x0112 from the first IFD of TIFF data (0: none).
func orientation(t []byte) uint16 {
	if len(t) < 8 {
		return 0
	}
	var o binary.ByteOrder = binary.BigEndian
	if string(t[:2]) == "II" {
		o = binary.LittleEndian
	}
	ifd := int64(o.Uint32(t[4:]))
	if ifd+2 > int64(len(t)) {
		return 0
	}
	n := int64(o.Uint16(t[ifd:]))
	for e := ifd + 2; e+12 <= int64(len(t)) && e < ifd+2+12*n; e += 12 {
		if o.Uint16(t[e:]) == 0x0112 {
			return o.Uint16(t[e+8:])
		}
	}
	return 0
}

// read reads [start, end), at most limit bytes (nil: cannot).
func read(r io.ReaderAt, start, end, limit int64) []byte {
	if start < 0 || end <= start {
		return nil
	}
	b := make([]byte, min(end-start, limit))
	if _, err := r.ReadAt(b, start); err != nil {
		return nil
	}
	return b
}

// walker reads a file's boxes or elements; all headers it reads count
// against its budget, so no input makes it work long.
type walker struct {
	r      io.ReaderAt
	budget int
}

// boxes calls f with the type and body of each MP4 box in [off, end)
// until f returns false.
func (w *walker) boxes(off, end int64, f func(typ string, start, end int64) bool) {
	var h [16]byte
	for ; off+8 <= end && w.budget > 0; w.budget-- {
		if _, err := w.r.ReadAt(h[:8], off); err != nil {
			return
		}
		size, head := int64(binary.BigEndian.Uint32(h[:4])), int64(8)
		switch size {
		case 0: // to the end
			size = end - off
		case 1: // 64-bit size
			if _, err := w.r.ReadAt(h[8:], off+8); err != nil {
				return
			}
			large := binary.BigEndian.Uint64(h[8:])
			if large > uint64(end-off) {
				return
			}
			size, head = int64(large), 16
		}
		if size < head || size > end-off || !f(string(h[4:8]), off+head, off+size) {
			return
		}
		off += size
	}
}

// mp4: moov → mvhd (length), first trak whose tkhd has a size.
func (w *walker) mp4(size int64) Info {
	var i Info
	w.boxes(0, size, func(typ string, start, end int64) bool {
		if typ != "moov" {
			return true
		}
		w.boxes(start, end, func(typ string, start, end int64) bool {
			switch {
			case typ == "mvhd":
				i.Duration = mvhdDuration(read(w.r, start, end, 32))
			case typ == "trak" && i.Width == 0:
				w.boxes(start, end, func(typ string, start, end int64) bool {
					if typ == "tkhd" {
						i.Width, i.Height = tkhdSize(read(w.r, start, end, 96))
					}
					return typ != "tkhd"
				})
			}
			return true
		})
		return false
	})
	return i
}

func mvhdDuration(b []byte) int64 {
	var scale, d uint64
	switch {
	case len(b) >= 20 && b[0] == 0:
		scale, d = uint64(binary.BigEndian.Uint32(b[12:])), uint64(binary.BigEndian.Uint32(b[16:]))
		if d == math.MaxUint32 {
			return 0 // unknown
		}
	case len(b) >= 32 && b[0] == 1:
		scale, d = uint64(binary.BigEndian.Uint32(b[20:])), binary.BigEndian.Uint64(b[24:])
	default:
		return 0
	}
	if scale == 0 || d/scale > math.MaxInt64/1000-1 {
		return 0
	}
	return int64(d/scale*1000 + d%scale*1000/scale)
}

// tkhdSize: width and height (16.16 fixed point) after the matrix, as
// players show them: turned by about 90 or 270 degrees, they swap.
func tkhdSize(b []byte) (w, h int) {
	at := 76 // version 0
	if len(b) > 0 && b[0] == 1 {
		at = 88
	}
	if len(b) < at+8 {
		return 0, 0
	}
	w, h = int(binary.BigEndian.Uint32(b[at:])>>16), int(binary.BigEndian.Uint32(b[at+4:])>>16)
	m := b[at-36:]
	if a, b := int32(binary.BigEndian.Uint32(m)), int32(binary.BigEndian.Uint32(m[4:])); abs(b) > abs(a) {
		w, h = h, w
	}
	return w, h
}

func abs(v int32) int64 { return max(int64(v), -int64(v)) }

// EBML ids (Matroska, WebM).
const (
	idEBML          = 0x1a45dfa3
	idSegment       = 0x18538067
	idInfo          = 0x1549a966
	idTimecodeScale = 0x2ad7b1
	idDuration      = 0x4489
	idTracks        = 0x1654ae6b
	idTrackEntry    = 0xae
	idVideo         = 0xe0
	idPixelWidth    = 0xb0
	idPixelHeight   = 0xba
	idDisplayWidth  = 0x54b0
	idDisplayHeight = 0x54ba
	idDisplayUnit   = 0x54b2
	idCluster       = 0x1f43b675
)

// elements calls f with the id and body of each EBML element in
// [off, end) until f returns false. An element of unknown size runs to
// end, and is the last.
func (w *walker) elements(off, end int64, f func(id uint32, start, end int64) bool) {
	for ; off < end && w.budget > 0; w.budget-- {
		id, start, size, ok := w.element(off, end)
		if !ok {
			return
		}
		if size < 0 { // unknown
			f(id, start, end)
			return
		}
		if !f(id, start, start+size) {
			return
		}
		off = start + size
	}
}

// element reads the header at off: id, body start and size (unknown:
// -1). ok is false for broken headers and sizes beyond end.
func (w *walker) element(off, end int64) (id uint32, start, size int64, ok bool) {
	var h [12]byte
	n, _ := w.r.ReadAt(h[:min(12, end-off)], off)
	b := h[:n]
	idLen := vintLen(b)
	if idLen == 0 || idLen > 4 {
		return 0, 0, 0, false
	}
	for _, c := range b[:idLen] {
		id = id<<8 | uint32(c)
	}
	b = b[idLen:]
	sizeLen := vintLen(b)
	if sizeLen == 0 {
		return 0, 0, 0, false
	}
	v := uint64(b[0]) & (0xff >> sizeLen)
	unknown := v == 0xff>>sizeLen
	for _, c := range b[1:sizeLen] {
		v, unknown = v<<8|uint64(c), unknown && c == 0xff
	}
	start = off + int64(idLen+sizeLen)
	switch {
	case unknown:
		return id, start, -1, true
	case v > uint64(end-start):
		return 0, 0, 0, false
	}
	return id, start, int64(v), true
}

// vintLen is the length of the variable-size integer b starts with (0:
// none, or longer than b).
func vintLen(b []byte) int {
	if len(b) == 0 || b[0] == 0 {
		return 0
	}
	if n := bits.LeadingZeros8(b[0]) + 1; n <= len(b) {
		return n
	}
	return 0
}

func (w *walker) uint(start, end int64) uint64 {
	var v uint64
	if end-start <= 8 {
		for _, c := range read(w.r, start, end, 8) {
			v = v<<8 | uint64(c)
		}
	}
	return v
}

func (w *walker) float(start, end int64) float64 {
	switch b := read(w.r, start, end, 8); len(b) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	}
	return 0
}

// webm: EBML header, Segment → Info (length), Tracks → first video track
// (pixel or display size). Info and Tracks come before the first Cluster.
func (w *walker) webm(size int64) Info {
	var i Info
	first := true
	w.elements(0, size, func(id uint32, start, end int64) bool {
		if first {
			first = false
			return id == idEBML
		}
		if id != idSegment {
			return true
		}
		w.elements(start, end, func(id uint32, start, end int64) bool {
			switch id {
			case idInfo:
				i.Duration = w.info(start, end)
			case idTracks:
				w.elements(start, end, func(id uint32, start, end int64) bool {
					if id == idTrackEntry {
						i.Width, i.Height = w.trackSize(start, end)
					}
					return i.Width == 0
				})
			}
			return id != idCluster
		})
		return false
	})
	return i
}

// info: Duration × TimecodeScale (nanoseconds) in milliseconds.
func (w *walker) info(start, end int64) int64 {
	scale, d := uint64(1_000_000), 0.0
	w.elements(start, end, func(id uint32, start, end int64) bool {
		switch id {
		case idTimecodeScale:
			scale = cmp.Or(w.uint(start, end), scale) // empty: the default
		case idDuration:
			d = w.float(start, end)
		}
		return true
	})
	ms := d * float64(scale) / 1e6
	if !(ms > 0 && ms <= maxDuration) { // also NaN
		return 0
	}
	return int64(math.Round(ms))
}

// trackSize: a track entry's video size, the display size if it has one
// in pixels (DisplayUnit 0) or as an aspect ratio (3).
func (w *walker) trackSize(start, end int64) (width, height int) {
	var pw, ph, dw, dh, unit uint64
	w.elements(start, end, func(id uint32, start, end int64) bool {
		if id != idVideo {
			return true
		}
		w.elements(start, end, func(id uint32, start, end int64) bool {
			switch v := w.uint(start, end); id {
			case idPixelWidth:
				pw = v
			case idPixelHeight:
				ph = v
			case idDisplayWidth:
				dw = v
			case idDisplayHeight:
				dh = v
			case idDisplayUnit:
				unit = v
			}
			return true
		})
		return false
	})
	switch {
	case dw == 0 || dh == 0 || unit != 0 && unit != 3:
	case unit == 0:
		pw, ph = dw, dh
	default:
		pw = uint64(math.Round(float64(ph) * float64(dw) / float64(dh)))
	}
	return side(pw), side(ph)
}

func side(v uint64) int {
	if v > maxSide {
		return 0
	}
	return int(v)
}
