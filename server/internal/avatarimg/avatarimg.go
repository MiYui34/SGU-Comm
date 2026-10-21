// Package avatarimg makes avatars from pictures (a fediverse account's):
// the same square JPEG as the web client's avatarEncode, so all avatars
// look alike.
package avatarimg

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif" // the first frame
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"kallisti.chat/server/internal/imagesize"
)

const (
	Size = 128 // pixels a side
	// Pictures read at most: panoramas pass, 64 MiB decoded at most.
	MaxSide   = 8192
	MaxPixels = 16 << 20
)

var ErrTooLarge = imagesize.ErrTooLarge

// background: the web client's, under transparent parts.
var background = color.RGBA{0x2b, 0x2d, 0x31, 0xff}

// Make turns a picture (JPEG, PNG, GIF or WebP) into an avatar: its middle
// square, Size pixels a side, on the background, as JPEG. The size in the
// header is checked before the picture is decoded.
func Make(data []byte) ([]byte, error) {
	if err := imagesize.Check(bytes.NewReader(data), MaxSide, MaxPixels); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, Size, Size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, squareCrop(src.Bounds()), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil // a few KB: within the Mumble server's limit
}

// squareCrop is the middle square of r (as the web client's squareCrop).
func squareCrop(r image.Rectangle) image.Rectangle {
	size := min(r.Dx(), r.Dy())
	x, y := r.Min.X+(r.Dx()-size)/2, r.Min.Y+(r.Dy()-size)/2
	return image.Rect(x, y, x+size, y+size)
}
