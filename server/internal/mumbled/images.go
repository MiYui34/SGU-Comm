package mumbled

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"kallisti.chat/server/internal/imagesize"
)

// Images in chat messages: Mumble clients send pictures as HTML with data:
// URLs (Murmur allows 128 KiB for such messages, 5000 for text). The
// server keeps only real images and builds the tags itself: no HTML from
// users goes through.

const (
	maxImageMessage = 128 << 10 // bytes, as Murmur's imagemessagelength
	maxImages       = 4
)

// Image is a picture from a chat message.
type Image struct {
	Type string // image/png, image/jpeg, image/gif or image/webp
	Data []byte
}

var (
	imgTag     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	dataSource = regexp.MustCompile(`(?is)\bsrc\s*=\s*["']?data:image/[a-z0-9.+-]+;base64,([a-z0-9+/=\s]+)`)
	imageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}
)

// extractImages takes the images out of a message: the rest (with its
// other HTML still in) and the images that really are images, by their
// bytes.
func extractImages(msg string) (string, []Image) {
	var images []Image
	rest := imgTag.ReplaceAllStringFunc(msg, func(tag string) string {
		m := dataSource.FindStringSubmatch(tag)
		if m == nil || len(images) == maxImages {
			return ""
		}
		// Padding is optional (some clients leave it out).
		data, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(strings.Join(strings.Fields(m[1]), ""), "="))
		if err != nil {
			return ""
		}
		typ := imageType(data)
		if typ == "" {
			return ""
		}
		// Clients show them inline: no pictures that take GBs to decode.
		switch err := imagesize.Check(bytes.NewReader(data), imagesize.MaxSide, imagesize.MaxPixels); {
		case errors.Is(err, imagesize.ErrTooLarge):
			return "[image too large]"
		case err != nil:
			return ""
		}
		images = append(images, Image{Type: typ, Data: data})
		return ""
	})
	return rest, images
}

// imageType is the type of data that is an allowed image ("": none).
func imageType(data []byte) string {
	if typ := http.DetectContentType(data); slices.Contains(imageTypes, typ) {
		return typ
	}
	return ""
}

// imagesHTML is the server's own tags for checked images.
func imagesHTML(images []Image) string {
	var b strings.Builder
	for _, img := range images {
		b.WriteString(`<img src="data:` + img.Type + `;base64,` + base64.StdEncoding.EncodeToString(img.Data) + `"/>`)
	}
	return b.String()
}
