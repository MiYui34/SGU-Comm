package mumbled

import (
	"fmt"
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
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...)
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
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 200)...)
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
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...)
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
	db.SetUserContent(alice, "", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...))
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
	set(&pb.UserState{Texture: append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 200)...)})
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
