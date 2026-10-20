package files

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/store"
)

func TestMediaType(t *testing.T) {
	ftyp := []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00")
	cases := []struct {
		head []byte
		name string
		want string
	}{
		{[]byte("ID3\x04\x00\x00"), "song.mp3", "audio/mpeg"},
		{[]byte{0xff, 0xfb, 0x90, 0x64}, "song.MP3", "audio/mpeg"}, // no ID3 tag
		{ftyp, "song.m4a", "audio/mp4"},
		{ftyp, "clip.mp4", "video/mp4"},
		{[]byte("OggS\x00\x02"), "voice.opus", "audio/ogg"},
		{[]byte("\x1a\x45\xdf\xa3\x01"), "clip.webm", "video/webm"},
		{[]byte("RIFF\x24\x00\x00\x00WAVEfmt "), "a.wav", "audio/wav"},
		{[]byte("fLaC\x00"), "a.flac", "audio/flac"},
		// Content and name must agree.
		{[]byte("ID3\x04"), "song.mp4", ""},
		{[]byte("<html>"), "song.mp3", ""},
		{[]byte{0xff, 0xe0, '<'}, "song.mp3", ""}, // reserved version and layer
		{ftyp, "clip.mov", ""},
	}
	for _, c := range cases {
		if got := mediaType(c.head, c.name); got != c.want {
			t.Errorf("mediaType(%q, %q) = %q, want %q", c.head, c.name, got, c.want)
		}
	}
}

func TestAudioAndVideoAreServedForPlaying(t *testing.T) {
	srv, s, h := setupHandler(t, 1<<20)
	mp3 := append([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"), make([]byte, 2000)...)
	_, v := upload(t, srv, "good", "song.mp3", "audio/mpeg", mp3)
	if v["type"] != "audio/mpeg" {
		t.Fatalf("response %v", v)
	}
	get, err := http.Get(srv.URL + v["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	get.Body.Close()
	if get.Header.Get("Content-Type") != "audio/mpeg" || !strings.HasPrefix(get.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("served as %v", get.Header)
	}
	// Seeking: a range.
	req, _ := http.NewRequest("GET", srv.URL+v["url"].(string), nil)
	req.Header.Set("Range", "bytes=100-199")
	part, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	part.Body.Close()
	if part.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: %d", part.StatusCode)
	}

	// An upload from before media types were kept: typed when served.
	key := "oldupload0000000000000000000000000000"
	s.AddUpload(store.Upload{Key: key, Name: "old.mp3", Type: "application/octet-stream", Size: int64(len(mp3)), Author: "alice"}, time.Now())
	os.WriteFile(filepath.Join(h.Dir, key), mp3, 0o600)
	old, err := http.Get(srv.URL + "/files/" + key + "/old.mp3")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(old.Body)
	old.Body.Close()
	if old.Header.Get("Content-Type") != "audio/mpeg" || !bytes.Equal(body, mp3) {
		t.Fatalf("old upload served as %v", old.Header)
	}
}

func TestUploadsKeepTheirSizes(t *testing.T) {
	srv, s, h := setupHandler(t, 1<<20)
	mp4, err := os.ReadFile("../mediasize/testdata/faststart.mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		data []byte
		want store.Media
	}{
		{"cat.png", pngBytes(t), store.Media{Width: 2, Height: 2}},
		{"clip.mp4", mp4, store.Media{Width: 64, Height: 48, Duration: 500}},
		{"clip.bin", mp4, store.Media{}}, // a download
	} {
		_, v := upload(t, srv, "good", c.name, "", c.data)
		if u, err := s.UploadByKey(v["key"].(string)); err != nil || u.Media != c.want {
			t.Errorf("%s: %+v %v", c.name, u, err)
		}
	}
	u, err := h.Save("bob", "a.png", "image/png", pngBytes(t))
	if err != nil || u.Media != (store.Media{Width: 2, Height: 2}) {
		t.Fatalf("saved %+v %v", u, err)
	}
}

func TestMeasureOldUploads(t *testing.T) {
	_, s, h := setupHandler(t, 1<<20)
	mp4, _ := os.ReadFile("../mediasize/testdata/rotated.mp4")
	key := "oldupload0000000000000000000000000000"
	// From before media types were kept: measured as served.
	old := store.Upload{Key: key, Name: "old.mp4", Type: "application/octet-stream", Size: int64(len(mp4))}
	os.WriteFile(filepath.Join(h.Dir, key), mp4, 0o600)
	if got := h.Measure(old); got != (store.Media{Width: 48, Height: 64, Duration: 500}) {
		t.Fatalf("old upload: %+v", got)
	}
	if got := h.Measure(store.Upload{Key: "gone", Name: "a.png", Type: "image/png"}); got != (store.Media{}) {
		t.Fatalf("missing file: %+v", got)
	}
	if got := h.Measure(store.Upload{Key: "../" + key, Name: "a.mp4", Type: "video/mp4"}); got != (store.Media{}) {
		t.Fatalf("path in the key: %+v", got)
	}
	s.AddUpload(old, time.Now())
	if err := s.MeasureUploads(h.Measure); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.UploadByKey(key); u.Width != 48 {
		t.Fatalf("stored %+v", u)
	}
}
