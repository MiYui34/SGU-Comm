package files

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kallisti.chat/server/internal/store"
)

// setupEmoji: the token "good" adds the emoji "party" (by alice).
func setupEmoji(t *testing.T) (*httptest.Server, *store.Store, *int) {
	t.Helper()
	_, s, h := setupHandler(t, 1<<20)
	changes := 0
	h.RedeemEmoji = func(token string) (string, string, bool) { return "alice", "party", token == "good" }
	h.OnEmoji = func() { changes++ }
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, s, &changes
}

func uploadEmoji(t *testing.T, srv *httptest.Server, token string, body []byte) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/api/emoji", bytes.NewReader(body))
	req.Header.Set("X-Upload-Token", token)
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

func TestUploadAndServeAnEmoji(t *testing.T) {
	srv, s, changes := setupEmoji(t)
	img := pngBytes(t)
	resp, v := uploadEmoji(t, srv, "good", img)
	if resp.StatusCode != http.StatusOK || v["name"] != "party" || *changes != 1 {
		t.Fatalf("status %d %v (changes %d)", resp.StatusCode, v, *changes)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Error("missing CORS header")
	}
	list, _ := s.Emojis()
	if len(list) != 1 || list[0].Name != "party" || list[0].Author != "alice" || list[0].Type != "image/png" {
		t.Fatalf("stored %+v", list)
	}
	get, err := http.Get(srv.URL + v["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	defer get.Body.Close()
	body, _ := io.ReadAll(get.Body)
	h := get.Header
	if get.StatusCode != http.StatusOK || !bytes.Equal(body, img) || h.Get("Content-Type") != "image/png" ||
		h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Content-Security-Policy") == "" ||
		h.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("serve %d %v", get.StatusCode, h)
	}
	for _, path := range []string{"/api/emoji/999", "/api/emoji/x", "/api/emoji/"} {
		if r, _ := http.Get(srv.URL + path); r.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", path, r.StatusCode)
		}
	}
}

func TestEmojiUploadsAreChecked(t *testing.T) {
	srv, s, changes := setupEmoji(t)
	big := append(pngBytes(t), make([]byte, MaxEmojiBytes)...)
	var hugePNG bytes.Buffer
	png.Encode(&hugePNG, image.NewGray(image.Rect(0, 0, 9000, 9000)))
	for _, c := range []struct {
		token  string
		body   []byte
		status int
	}{
		{"bad", pngBytes(t), http.StatusUnauthorized},
		{"good", []byte("<svg onload=alert(1)>"), http.StatusUnsupportedMediaType},
		{"good", []byte{}, http.StatusUnsupportedMediaType},
		{"good", big, http.StatusRequestEntityTooLarge},
		{"good", hugePNG.Bytes(), http.StatusRequestEntityTooLarge}, // too many pixels
		{"good", pngBytes(t)[:20], http.StatusUnsupportedMediaType}, // broken
	} {
		if resp, v := uploadEmoji(t, srv, c.token, c.body); resp.StatusCode != c.status || v["error"] == nil {
			t.Errorf("%s %d bytes: %d %v", c.token, len(c.body), resp.StatusCode, v)
		}
	}
	// The name was taken meanwhile.
	s.AddEmoji(store.Emoji{Name: "party", Type: "image/png", Data: []byte("x")}, time.Now())
	if resp, _ := uploadEmoji(t, srv, "good", pngBytes(t)); resp.StatusCode != http.StatusConflict {
		t.Errorf("taken: %d", resp.StatusCode)
	}
	if *changes != 0 {
		t.Errorf("%d changes", *changes)
	}
}
