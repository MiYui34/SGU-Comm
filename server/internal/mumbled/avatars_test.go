package mumbled

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAvatarsOfRegisteredUsers(t *testing.T) {
	db := testDB(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...)
	alice, _ := db.RegisterUser("Alice", "aa")
	db.SetUserContent(alice, "", png)
	bob, _ := db.RegisterUser("bob", "bb")
	db.SetUserContent(bob, "", []byte("<svg onload=alert(1)>"))
	h := db.Avatars()
	get := func(path, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if etag != "" {
			r.Header.Set("If-None-Match", etag)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := get("/api/avatars/alice", "") // names compare as Mumble does
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.Len() != len(png) {
		t.Fatalf("alice: %d %v", w.Code, w.Header())
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("ETag") == "" {
		t.Fatalf("headers %v", w.Header())
	}
	if again := get("/api/avatars/alice", w.Header().Get("ETag")); again.Code != http.StatusNotModified {
		t.Fatalf("revalidation: %d", again.Code)
	}
	for _, path := range []string{"/api/avatars/bob", "/api/avatars/nobody", "/api/avatars/"} {
		if w := get(path, ""); w.Code != 404 || w.Header().Get("Cache-Control") != "max-age=300" {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}
