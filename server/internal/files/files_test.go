package files

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/store"
)

func pngBytes(t *testing.T) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func setup(t *testing.T, maxBytes int64) (*httptest.Server, *store.Store) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	h := &Handler{
		Store:          s,
		Dir:            t.TempDir(),
		MaxBytes:       maxBytes,
		AllowedOrigins: []string{"https://app.example"},
		Now:            func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) },
		Redeem: func(token string) (string, bool) {
			return "alice", token == "good"
		},
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, s
}

func upload(t *testing.T, srv *httptest.Server, token, name, ctype string, body []byte) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/api/upload?token="+token, bytes.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("X-Filename", url.QueryEscape(name))
	req.Header.Set("Origin", "https://app.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v map[string]any
	json.NewDecoder(resp.Body).Decode(&v)
	return resp, v
}

func TestUploadAndServeAnImage(t *testing.T) {
	srv, s := setup(t, 1<<20)
	img := pngBytes(t)
	resp, v := upload(t, srv, "good", "my cat.png", "image/png", img)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d %v", resp.StatusCode, v)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Error("missing CORS header")
	}
	key, _ := v["key"].(string)
	if len(key) < 32 || v["name"] != "my cat.png" || v["type"] != "image/png" || v["size"] != float64(len(img)) {
		t.Fatalf("response %v", v)
	}
	u, err := s.UploadByKey(key)
	if err != nil || u.Author != "alice" {
		t.Fatalf("stored %+v %v", u, err)
	}
	get, err := http.Get(srv.URL + v["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	defer get.Body.Close()
	data, _ := io.ReadAll(get.Body)
	if get.StatusCode != 200 || !bytes.Equal(data, img) {
		t.Fatalf("serve: %d, %d bytes", get.StatusCode, len(data))
	}
	h := get.Header
	if h.Get("Content-Type") != "image/png" || !strings.HasPrefix(h.Get("Content-Disposition"), "inline") ||
		h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("headers: %v", h)
	}
}

func TestNonImagesAreDownloadsEvenIfTheyClaimOtherwise(t *testing.T) {
	srv, _ := setup(t, 1<<20)
	html := []byte("<html><script>alert(1)</script></html>")
	for _, ctype := range []string{"text/html", "image/png", "image/svg+xml"} {
		_, v := upload(t, srv, "good", "x.html", ctype, html)
		get, err := http.Get(srv.URL + v["url"].(string))
		if err != nil {
			t.Fatal(err)
		}
		get.Body.Close()
		if get.Header.Get("Content-Type") != "application/octet-stream" ||
			!strings.HasPrefix(get.Header.Get("Content-Disposition"), "attachment") {
			t.Errorf("%s: served as %q, %q", ctype, get.Header.Get("Content-Type"), get.Header.Get("Content-Disposition"))
		}
	}
}

func TestUploadRefusals(t *testing.T) {
	srv, _ := setup(t, 100)
	if resp, _ := upload(t, srv, "bad", "a.txt", "text/plain", []byte("x")); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad token: %d", resp.StatusCode)
	}
	if resp, _ := upload(t, srv, "good", "a.bin", "application/octet-stream", make([]byte, 101)); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("too large: %d", resp.StatusCode)
	}
	if resp, _ := upload(t, srv, "good", "empty.txt", "text/plain", nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty: %d", resp.StatusCode)
	}
}

func TestUnknownFileIs404(t *testing.T) {
	srv, _ := setup(t, 100)
	resp, err := http.Get(srv.URL + "/files/nope/x.png")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestPreflight(t *testing.T) {
	srv, _ := setup(t, 100)
	req, _ := http.NewRequest("OPTIONS", srv.URL+"/api/upload", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "X-Filename") {
		t.Fatalf("preflight: %d %v", resp.StatusCode, resp.Header)
	}
	req.Header.Set("Origin", "https://evil.example")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS allowed for an unknown origin")
	}
}

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd":       "passwd",
		"a\x00b\nc.txt":          "abc.txt",
		"":                       "file",
		strings.Repeat("x", 300): strings.Repeat("x", 200),
	}
	for in, want := range cases {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
