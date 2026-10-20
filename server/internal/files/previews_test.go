package files

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kallisti.chat/server/internal/store"
	"kallisti.chat/server/internal/thumbs"
)

func TestPreviewJob(t *testing.T) {
	big := store.Media{Width: 2000, Height: 1000}
	for _, c := range []struct {
		name   string
		u      store.Upload
		format thumbs.Format
		ok     bool
		max    int
		seek   bool
	}{
		{"video", store.Upload{Type: "video/mp4", Media: store.Media{Duration: 5000}}, thumbs.WebP, true, 480, true},
		{"short video: first frame", store.Upload{Type: "video/webm", Media: store.Media{Duration: 1500}}, thumbs.WebP, true, 480, false},
		{"video of unknown length", store.Upload{Type: "video/webm"}, thumbs.JPEG, true, 480, false},
		{"large png", store.Upload{Type: "image/png", Size: 1000, Media: big}, thumbs.WebP, true, 1000, false},
		{"large png, no transparency", store.Upload{Type: "image/png", Size: 1000, Media: big}, thumbs.JPEG, false, 0, false},
		{"large jpeg", store.Upload{Type: "image/jpeg", Size: 1000, Media: big}, thumbs.JPEG, true, 1000, false},
		{"heavy webp", store.Upload{Type: "image/webp", Size: 2 << 20, Media: store.Media{Width: 800, Height: 600}}, thumbs.WebP, true, 1000, false},
		{"small png", store.Upload{Type: "image/png", Size: 1000, Media: store.Media{Width: 800, Height: 600}}, thumbs.WebP, false, 0, false},
		{"gif keeps its animation", store.Upload{Type: "image/gif", Size: 5 << 20, Media: big}, thumbs.WebP, false, 0, false},
		{"picture of unknown size", store.Upload{Type: "image/png", Size: 5 << 20}, thumbs.WebP, false, 0, false},
		{"audio", store.Upload{Type: "audio/mp4", Media: store.Media{Duration: 5000}}, thumbs.WebP, false, 0, false},
		{"download", store.Upload{Type: "application/octet-stream", Size: 5 << 20}, thumbs.WebP, false, 0, false},
	} {
		c.u.Key = "k"
		j, ok := previewJob(c.u, "/up", c.format)
		if ok != c.ok || ok && (j.Max != c.max || j.Seek != c.seek || j.Src != "/up/k" || j.Dst != "/up/k.preview" || j.Type != c.u.Type) {
			t.Errorf("%s: %+v %v", c.name, j, ok)
		}
	}
}

// fakeTool is a shell script acting as an ffmpeg that writes WebP.
func fakeTool(t *testing.T, body string) thumbs.Tool {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\nfor a; do out=$a; done; out=${out#file:}\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return thumbs.Tool{Command: []string{path}, Format: thumbs.WebP}
}

func bigPNG(t *testing.T) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1700, 100))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withPreviews gives the handler a maker running the fake ffmpeg; the
// channel gets the keys of uploads with a new preview.
func withPreviews(t *testing.T, h *Handler, body string) chan string {
	ready := make(chan string, 4)
	h.Previews = thumbs.New(t.Context(), fakeTool(t, body), 2, 8)
	h.OnPreview = func(key string) { ready <- key }
	return ready
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// waitFor waits for the preview of an upload to be made, or refused.
func waitFor(t *testing.T, s *store.Store, key string) store.Upload {
	t.Helper()
	for range 500 {
		if u, _ := s.UploadByKey(key); u.Preview != "" {
			return u
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no preview made")
	return store.Upload{}
}

func TestUploadGetsAPreview(t *testing.T) {
	srv, s, h := setupHandler(t, 1<<20)
	ready := withPreviews(t, h, `printf thumb > "$out"`)
	_, v := upload(t, srv, "good", "big.png", "image/png", bigPNG(t))
	key := v["key"].(string)
	if got := <-ready; got != key {
		t.Fatalf("ready: %s", got)
	}
	if u, _ := s.UploadByKey(key); u.Preview != "image/webp" {
		t.Fatalf("stored %+v", u)
	}
	resp, body := get(t, srv.URL+v["url"].(string)+"?preview")
	hd := resp.Header
	if resp.StatusCode != 200 || body != "thumb" || hd.Get("Content-Type") != "image/webp" ||
		hd.Get("X-Content-Type-Options") != "nosniff" || hd.Get("Content-Security-Policy") == "" {
		t.Fatalf("preview: %d %q %v", resp.StatusCode, body, hd)
	}
	if resp, body := get(t, srv.URL+v["url"].(string)); resp.StatusCode != 200 || body == "thumb" {
		t.Fatalf("the original: %d %q", resp.StatusCode, body)
	}
	// Mumble pictures too.
	u, err := h.Save("bob", "a.png", "image/png", bigPNG(t))
	if err != nil || <-ready != u.Key {
		t.Fatalf("saved: %v", err)
	}
	// Deleted with the upload.
	h.Remove(key)
	if _, err := os.Stat(filepath.Join(h.Dir, key+".preview")); !os.IsNotExist(err) {
		t.Fatalf("preview left: %v", err)
	}
}

func TestPreviewAccess(t *testing.T) {
	srv, s, h := setupHandler(t, 1<<20)
	_, v := upload(t, srv, "good", "small.png", "image/png", pngBytes(t))
	key := v["key"].(string)
	// A preview file without the upload having one is not served, nor
	// one of an unknown upload.
	os.WriteFile(filepath.Join(h.Dir, key+".preview"), []byte("x"), 0o600)
	for _, path := range []string{v["url"].(string), "/files/nokey/a.png", "/files/" + key + ".preview/a.png", "/files/..%2F" + key + "/a.png"} {
		if resp, _ := get(t, srv.URL+path+"?preview"); resp.StatusCode != 404 {
			t.Errorf("%s: %d", path, resp.StatusCode)
		}
	}
	s.SetUploadPreview(key, store.NoPreview)
	if resp, _ := get(t, srv.URL+v["url"].(string)+"?preview"); resp.StatusCode != 404 {
		t.Errorf("refused preview: %d", resp.StatusCode)
	}
	s.SetUploadPreview(key, "image/webp")
	if resp, _ := get(t, srv.URL+v["url"].(string)+"?preview"); resp.StatusCode != 200 {
		t.Errorf("made preview: %d", resp.StatusCode)
	}
	// Unused uploads go with their previews.
	h.Now = func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) }
	h.DeleteUnused()
	if entries, _ := os.ReadDir(h.Dir); len(entries) != 0 {
		t.Fatalf("files left: %v", entries)
	}
	if resp, _ := get(t, srv.URL+v["url"].(string)+"?preview"); resp.StatusCode != 404 {
		t.Errorf("deleted upload: %d", resp.StatusCode)
	}
}

func TestNoFFmpegNoPreviews(t *testing.T) {
	srv, s, h := setupHandler(t, 1<<20)
	_, v := upload(t, srv, "good", "big.png", "image/png", bigPNG(t))
	h.MakeMissingPreviews(t.Context())
	if u, _ := s.UploadByKey(v["key"].(string)); u.Preview != "" {
		t.Fatalf("stored %+v", u)
	}
}

func TestFailedPreviewIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"ffmpeg fails":         `printf partial > "$out"; exit 1`,
		"larger than the file": `head -c 2000000 /dev/zero > "$out"`,
	} {
		srv, s, h := setupHandler(t, 1<<20)
		ready := withPreviews(t, h, body)
		_, v := upload(t, srv, "good", "big.png", "image/png", bigPNG(t))
		key := v["key"].(string)
		if u := waitFor(t, s, key); u.Preview != store.NoPreview {
			t.Errorf("%s: stored %+v", name, u)
		}
		if _, err := os.Stat(filepath.Join(h.Dir, key+".preview")); !os.IsNotExist(err) {
			t.Errorf("%s: preview file: %v", name, err)
		}
		if len(ready) != 0 {
			t.Errorf("%s: announced", name)
		}
	}
}

func TestPreviewOfDeletedUploadIsRemoved(t *testing.T) {
	_, _, h := setupHandler(t, 1<<20)
	ready := withPreviews(t, h, `printf thumb > "$out"`)
	u := store.Upload{Key: "gone0000000000000000000000000000", Type: "image/png", Size: 5 << 20, Media: store.Media{Width: 2000, Height: 1000}}
	os.WriteFile(filepath.Join(h.Dir, u.Key+".preview"), []byte("thumb"), 0o600)
	h.previewDone(u, "image/webp", nil)
	if entries, _ := os.ReadDir(h.Dir); len(entries) != 0 || len(ready) != 0 {
		t.Fatalf("files left: %v", entries)
	}
}

func TestMakeMissingPreviews(t *testing.T) {
	_, s, h := setupHandler(t, 1<<20)
	ready := withPreviews(t, h, `printf thumb > "$out"`)
	s.EnsureChannels([]string{"general"})
	mp4, _ := os.ReadFile("../mediasize/testdata/faststart.mp4")
	key := "old00000000000000000000000000000"
	os.WriteFile(filepath.Join(h.Dir, key), mp4, 0o600)
	old, _ := s.AddUpload(store.Upload{Key: key, Name: "a.mp4", Type: "video/mp4", Size: int64(len(mp4)), Author: "a"}, time.Now())
	s.AddMessage(store.NewMessage{ChannelID: 1, Author: "a", At: time.Now(), UploadID: old.ID})
	h.MakeMissingPreviews(t.Context())
	if u, _ := s.UploadByKey(key); u.Preview != "image/webp" || len(ready) != 1 {
		t.Fatalf("stored %+v", u)
	}
}
