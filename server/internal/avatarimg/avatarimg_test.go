package avatarimg

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// halves: the left half red, the right half blue (a wide picture shows
// whether the middle is kept).
func halves(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{255, 0, 0, 255}
			if x >= w/2 {
				c = color.NRGBA{0, 0, 255, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func encode(t *testing.T, f func(*bytes.Buffer) error) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := f(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngOf(t *testing.T, img image.Image) []byte {
	return encode(t, func(b *bytes.Buffer) error { return png.Encode(b, img) })
}

func wantColor(t *testing.T, name string, img image.Image, x, y int, want color.RGBA) {
	t.Helper()
	r, g, b, _ := img.At(x, y).RGBA()
	near := func(got uint32, want uint8) bool { d := int(got>>8) - int(want); return d > -24 && d < 24 }
	if !near(r, want.R) || !near(g, want.G) || !near(b, want.B) {
		t.Errorf("%s: (%d,%d) is %d,%d,%d, want about %v", name, x, y, r>>8, g>>8, b>>8, want)
	}
}

func mustMake(t *testing.T, name string, data []byte) image.Image {
	t.Helper()
	out, err := Make(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(out) > 120<<10 { // the most the Mumble server takes

		t.Fatalf("%s: %d bytes", name, len(out))
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("%s: not a JPEG: %v", name, err)
	}
	if b := img.Bounds(); b.Dx() != Size || b.Dy() != Size {
		t.Fatalf("%s: %v, want %dx%d", name, b, Size, Size)
	}
	return img
}

var (
	red   = color.RGBA{255, 0, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
	green = color.RGBA{0, 255, 0, 255}
)

func TestMake(t *testing.T) {
	wide := halves(400, 100) // the middle square: half red, half blue
	for name, data := range map[string][]byte{
		"png":  pngOf(t, wide),
		"jpeg": encode(t, func(b *bytes.Buffer) error { return jpeg.Encode(b, wide, &jpeg.Options{Quality: 95}) }),
	} {
		img := mustMake(t, name, data)
		wantColor(t, name, img, 10, 64, red)
		wantColor(t, name, img, 117, 64, blue)
	}
	tall := halves(100, 400) // the middle square of a tall one
	img := mustMake(t, "tall", pngOf(t, tall))
	wantColor(t, "tall", img, 10, 10, red)
	wantColor(t, "tall", img, 117, 117, blue)

	pal := image.NewPaletted(image.Rect(0, 0, 50, 80), []color.Color{green})
	img = mustMake(t, "gif", encode(t, func(b *bytes.Buffer) error { return gif.Encode(b, pal, nil) }))
	wantColor(t, "gif", img, 64, 64, green)

	// Transparent parts are on the web client's background.
	img = mustMake(t, "transparent", pngOf(t, image.NewNRGBA(image.Rect(0, 0, 10, 10))))
	wantColor(t, "transparent", img, 64, 64, color.RGBA{0x2b, 0x2d, 0x31, 255})
}

func TestMakeRefuses(t *testing.T) {
	for name, img := range map[string]image.Image{
		"too wide":        image.NewGray(image.Rect(0, 0, MaxSide+1, 1)),
		"too many pixels": image.NewGray(image.Rect(0, 0, 4097, 4097)),
	} {
		if _, err := Make(pngOf(t, img)); !errors.Is(err, ErrTooLarge) {
			t.Errorf("%s: %v", name, err)
		}
	}
	small := pngOf(t, halves(4, 4))
	for name, data := range map[string][]byte{"empty": nil, "text": []byte("<svg/>"), "cut": small[:len(small)/2]} {
		if _, err := Make(data); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
