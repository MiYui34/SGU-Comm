package files

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
)

// Audio and video that browsers play inline. A type is given only when
// the content's signature and the file name's extension agree, so nothing
// else is served as media. Keep the extensions in step with the web app's
// lib/media.ts (older uploads play by name there).
type mediaFormat struct {
	exts  []string
	magic func(head []byte) bool
	ctype string
}

var media = []mediaFormat{
	{[]string{".mp3"}, isMP3, "audio/mpeg"},
	{[]string{".m4a"}, isMP4, "audio/mp4"},
	{[]string{".mp4", ".m4v"}, isMP4, "video/mp4"},
	{[]string{".ogg", ".oga", ".opus"}, prefix("OggS"), "audio/ogg"},
	{[]string{".webm"}, prefix("\x1a\x45\xdf\xa3"), "video/webm"},
	{[]string{".wav"}, isWAV, "audio/wav"},
	{[]string{".flac"}, prefix("fLaC"), "audio/flac"},
}

// mediaTypes are the types mediaType gives (served inline).
var mediaTypes = func() []string {
	out := make([]string, len(media))
	for i, m := range media {
		out[i] = m.ctype
	}
	return out
}()

func mediaType(head []byte, name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	for _, m := range media {
		if !slices.Contains(m.exts, ext) {
			continue
		}
		if m.magic(head) {
			return m.ctype
		}
	}
	return ""
}

// mediaName: the name has an extension mediaType knows (worth reading the
// file's start for).
func mediaName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return slices.ContainsFunc(media, func(m mediaFormat) bool { return slices.Contains(m.exts, ext) })
}

// isInline: served for showing or playing, not as a download.
func isInline(ctype string) bool {
	return slices.Contains(inlineTypes, ctype) || slices.Contains(mediaTypes, ctype)
}

func prefix(p string) func([]byte) bool {
	return func(head []byte) bool { return bytes.HasPrefix(head, []byte(p)) }
}

// isMP3: an ID3 tag, or an MPEG audio frame header (sync, a version and a
// layer that are not reserved).
func isMP3(head []byte) bool {
	return bytes.HasPrefix(head, []byte("ID3")) ||
		len(head) >= 2 && head[0] == 0xff && head[1]&0xe0 == 0xe0 && head[1]&0x18 != 0x08 && head[1]&0x06 != 0
}

func isMP4(head []byte) bool { return len(head) >= 8 && string(head[4:8]) == "ftyp" }

func isWAV(head []byte) bool {
	return len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE"
}
