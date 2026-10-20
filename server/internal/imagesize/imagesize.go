// Package imagesize checks a picture's size from its header: a few KB of
// PNG can hold a picture that takes gigabytes to decode in a browser.
package imagesize

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

// Limits for images that clients show inline (chat images, link
// previews): panoramas and large phone photos pass, decompression bombs
// do not (50 MP is 200 MB decoded).
const (
	MaxSide   = 16384
	MaxPixels = 50_000_000
)

// Fits: r is an image (PNG, JPEG, GIF or WebP) of at most maxSide pixels a
// side and maxPixels in all. Only the header is read.
func Fits(r io.Reader, maxSide, maxPixels int) bool {
	c, _, err := image.DecodeConfig(r)
	return err == nil && c.Width <= maxSide && c.Height <= maxSide && c.Width*c.Height <= maxPixels
}

// Inline: r is an image within the limits for inline images.
func Inline(r io.Reader) bool { return Fits(r, MaxSide, MaxPixels) }
