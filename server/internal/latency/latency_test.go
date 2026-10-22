package latency_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"kallisti.chat/server/internal/latency"
)

func TestGetAndHead(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			latency.Handler().ServeHTTP(w, httptest.NewRequest(method, "/api/ping", nil))

			if w.Code != http.StatusNoContent {
				t.Fatalf("status: want %d, got %d", http.StatusNoContent, w.Code)
			}
			if body := w.Body.Len(); body != 0 {
				t.Fatalf("body: want empty, got %d bytes", body)
			}
			if cache := w.Header().Get("Cache-Control"); cache != "no-store" {
				t.Fatalf("cache-control: want no-store, got %q", cache)
			}
		})
	}
}

func TestOtherMethods(t *testing.T) {
	w := httptest.NewRecorder()
	latency.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/ping", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: want %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("allow: want %q, got %q", "GET, HEAD", allow)
	}
}
