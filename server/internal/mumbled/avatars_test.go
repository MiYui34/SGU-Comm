package mumbled

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func TestAvatarsOfRegisteredUsers(t *testing.T) {
	db := testDB(t)
	png := pngOf(t, 4, 4)
	alice, _ := db.RegisterUser("Alice", "aa")
	db.SetUserContent(alice, "", png)
	bob, _ := db.RegisterUser("bob", "bb")
	db.SetUserContent(bob, "", []byte("<svg onload=alert(1)>"))
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	h := s.Avatars()
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
	// With the picture's hash (as clients know it from UserState): kept.
	etag := w.Header().Get("ETag")
	if w := get("/api/avatars/alice?v="+etag[1:len(etag)-1], ""); w.Code != 200 || w.Header().Get("Cache-Control") != "max-age=31536000, immutable" {
		t.Fatalf("hashed: %d %v", w.Code, w.Header())
	}
	if w := get("/api/avatars/alice?v=00", ""); w.Code != 200 || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("old hash: %d %v", w.Code, w.Header())
	}
	for _, path := range []string{"/api/avatars/bob", "/api/avatars/nobody", "/api/avatars/"} {
		if w := get(path, ""); w.Code != 404 || w.Header().Get("Cache-Control") != "max-age=300" {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

// Online users' avatars are the live ones: clients ask as soon as the
// hash comes, maybe before the save.
func TestAvatarsOfOnlineUsersAreLive(t *testing.T) {
	db := testDB(t)
	events := make(chan string, 10)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnPictureChanged: func(id uint32, _ bool) {
		events <- fmt.Sprint(id)
	}}, nil)
	alice, id := registered(t, addr, "alice")
	png := pngOf(t, 64, 64)
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), Texture: png})
	expectEvent(t, events, fmt.Sprint(id))
	db.SetUserContent(id, "", []byte("\x89PNG\r\n\x1a\nold")) // as before the save
	w := httptest.NewRecorder()
	s.Avatars().ServeHTTP(w, httptest.NewRequest("GET", "/api/avatars/alice", nil))
	if w.Code != 200 || w.Body.Len() != len(png) {
		t.Fatalf("%d %d", w.Code, w.Body.Len())
	}
}

func TestPicturesAreFlaggedWhenSaved(t *testing.T) {
	db := testDB(t)
	png := pngOf(t, 4, 4)
	alice, _ := db.RegisterUser("alice", "aa")
	bob, _ := db.RegisterUser("bob", "bb")
	db.RegisterUser("carol", "cc") // none
	db.SetUserContent(alice, "", png)
	db.SetUserContent(bob, "", []byte("<svg onload=alert(1)>")) // no image
	if got, err := db.Pictures(); err != nil || !maps.Equal(got, map[uint32]bool{alice: true}) {
		t.Fatalf("pictures %v %v", got, err)
	}
	db.SetUserContent(alice, "bio", nil)
	if got, _ := db.Pictures(); len(got) != 0 {
		t.Fatalf("after removal %v", got)
	}
}

// Databases from before the flag get it for the pictures they have.
func TestOldPicturesAreFlagged(t *testing.T) {
	db := testDB(t)
	alice, _ := db.RegisterUser("alice", "aa")
	db.SetUserContent(alice, "", pngOf(t, 4, 4))
	if _, err := db.db.Exec(`ALTER TABLE mumble_users DROP COLUMN picture`); err != nil {
		t.Fatal(err)
	}
	db2, err := OpenDB(db.db)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := db2.Pictures(); !maps.Equal(got, map[uint32]bool{alice: true}) {
		t.Fatalf("pictures %v", got)
	}
}

// Flags set before the size check are checked again once: an oversized
// or broken avatar loses its flag, real pictures keep theirs.
func TestPictureFlagsAreRecheckedOnce(t *testing.T) {
	db := testDB(t)
	alice, _ := db.RegisterUser("alice", "aa")
	bob, _ := db.RegisterUser("bob", "bb")
	carol, _ := db.RegisterUser("carol", "cc")
	db.SetUserContent(alice, "", pngOf(t, 4, 4))
	// As older servers saved them: flagged without the size check.
	db.db.Exec(`UPDATE mumble_users SET texture = ?, picture = 1 WHERE id = ?`, pngOf(t, 4000, 4000), bob)
	db.db.Exec(`UPDATE mumble_users SET texture = ?, picture = 1 WHERE id = ?`, []byte("\x89PNG\r\n\x1a\nbroken"), carol)
	db.db.Exec(`DELETE FROM mumble_meta WHERE key = 'pictures_checked'`)
	db2, err := OpenDB(db.db)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := db2.Pictures(); !maps.Equal(got, map[uint32]bool{alice: true}) {
		t.Fatalf("pictures %v", got)
	}
	// Once: a flag set later (as by hand) stays.
	db.db.Exec(`UPDATE mumble_users SET picture = 1 WHERE id = ?`, bob)
	if _, err := OpenDB(db.db); err != nil {
		t.Fatal(err)
	}
	if got, _ := db2.Pictures(); len(got) != 2 {
		t.Fatalf("checked again: %v", got)
	}
}

// Saved pictures that are set, changed or removed are reported (true:
// there is one, an image).
func TestPictureChangesAreReported(t *testing.T) {
	db := testDB(t)
	setAdminPassword(db, "admin")
	events := make(chan string, 10)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, OnPictureChanged: func(id uint32, picture bool) {
		events <- fmt.Sprint(id, " ", picture)
	}}, nil)
	alice, id := registered(t, addr, "alice")
	set := func(m *pb.UserState) {
		m.Session = proto.Uint32(alice.session)
		alice.send(mumble.TypeUserState, m)
	}
	set(&pb.UserState{Texture: []byte("<svg onload=alert(1)>")}) // no image
	expectEvent(t, events, fmt.Sprint(id, " false"))
	set(&pb.UserState{Texture: pngOf(t, 64, 64)})
	expectEvent(t, events, fmt.Sprint(id, " true"))
	if got := s.Pictures(); !maps.Equal(got, map[uint32]bool{id: true}) {
		t.Fatalf("pictures %v", got)
	}
	set(&pb.UserState{Comment: proto.String("hi")}) // no picture change
	set(&pb.UserState{Texture: []byte{}})
	expectEvent(t, events, fmt.Sprint(id, " false"))
	if got := s.Pictures(); len(got) != 0 {
		t.Fatalf("after removal %v", got)
	}
}

// pngOf is a real (blank) PNG picture of this size: a few hundred bytes
// even when large.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// webpOf is the start of a lossless WebP of this size (all that a size
// check reads).
func webpOf(w, h int) []byte {
	bits := uint32(w-1) | uint32(h-1)<<14
	chunk := append([]byte("VP8L\x05\x00\x00\x00\x2f"), byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24), 0)
	return append(append([]byte("RIFF"), byte(4+len(chunk)), 0, 0, 0), append([]byte("WEBP"), chunk...)...)
}

// Avatars are at most 1024 px a side (as clients show them): a small
// file can hold a huge picture.
func TestAvatarTypeChecksTheSize(t *testing.T) {
	encoded := func(enc func(*bytes.Buffer, image.Image) error, w, h int) []byte {
		var b bytes.Buffer
		if err := enc(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	gifOf := func(b *bytes.Buffer, m image.Image) error { return gif.Encode(b, m, nil) }
	jpegOf := func(b *bytes.Buffer, m image.Image) error { return jpeg.Encode(b, m, nil) }
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		{"png", pngOf(t, 1024, 1024), "image/png"},
		{"png too wide", pngOf(t, 1025, 1), ""},
		{"gif", encoded(gifOf, 8, 8), "image/gif"},
		{"gif too high", encoded(gifOf, 1, 5000), ""},
		{"jpeg", encoded(jpegOf, 8, 8), "image/jpeg"},
		{"jpeg too large", encoded(jpegOf, 2000, 16), ""},
		{"webp", webpOf(16, 16), "image/webp"},
		{"webp too large", webpOf(4000, 16), ""},
		{"only the signature", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...), ""},
		{"no image", []byte("<svg onload=alert(1)>"), ""},
	} {
		if got := avatarType(c.data); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestOversizedAvatarsAreNotServed(t *testing.T) {
	db := testDB(t)
	bob, _ := db.RegisterUser("bob", "bb")
	db.SetUserContent(bob, "", pngOf(t, 4000, 4000))
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	alice, _ := registered(t, addr, "alice")
	alice.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(alice.session), Texture: pngOf(t, 1, 3000)})
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.state.Users[alice.session].Texture) > 0
	})
	for _, name := range []string{"alice", "bob"} { // online, offline
		w := httptest.NewRecorder()
		s.Avatars().ServeHTTP(w, httptest.NewRequest("GET", "/api/avatars/"+name, nil))
		if w.Code != 404 {
			t.Fatalf("%s: %d", name, w.Code)
		}
	}
	if got, _ := db.Pictures(); len(got) != 0 {
		t.Fatalf("flagged %v", got)
	}
}
