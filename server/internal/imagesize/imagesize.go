// Package imagesize checks a picture's size from its header: a few KB of
// PNG can hold a picture that takes gigabytes to decode in a browser.
package imagesize

import (
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

// Limits for images that clients show inline (chat images, link
// previews): panoramas and "50 MP" phone photos (8192x6144) pass,
// decompression bombs do not (64 Mi pixels are 256 MiB decoded).
const (
	MaxSide   = 16384
	MaxPixels = 1 << 26
)

var ErrTooLarge = errors.New("image too large")

// Check: r is an image (PNG, JPEG, GIF or WebP) of at most maxSide pixels
// a side and maxPixels in all (else ErrTooLarge, or the decode error).
// Only the header is read.
func Check(r io.Reader, maxSide, maxPixels int) error {
	c, _, err := image.DecodeConfig(r)
	if err == nil && (c.Width > maxSide || c.Height > maxSide || c.Width*c.Height > maxPixels) {
		err = ErrTooLarge
	}
	return err
}

func Fits(r io.Reader, maxSide, maxPixels int) bool { return Check(r, maxSide, maxPixels) == nil }

// Inline checks an image against the limits for inline images.
func Inline(r io.Reader) error { return Check(r, MaxSide, MaxPixels) }
