package imagesize

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestInline(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want bool
	}{
		{"small", pngOf(t, 16, 16), true},
		{"widest", pngOf(t, MaxSide, 1), true},
		{"50 MP phone photo", pngOf(t, 8192, 6144), true},
		{"too wide", pngOf(t, MaxSide+1, 1), false},
		{"too many pixels", pngOf(t, 9000, 9000), false},
		{"only the signature", []byte("\x89PNG\r\n\x1a\n"), false},
		{"no image", []byte("<svg/>"), false},
	} {
		if got := Inline(bytes.NewReader(c.data)) == nil; got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}
