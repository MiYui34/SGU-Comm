package mumbled

import (
	"bytes"
	"fmt"
	"net/netip"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func (s *Server) savedTexture(t *testing.T, certHash string) []byte {
	t.Helper()
	r, err := s.cfg.DB.UserByHash(certHash)
	if err != nil || r == nil {
		t.Fatalf("%s: not registered (%v)", certHash, err)
	}
	_, texture, err := s.cfg.DB.UserContent(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	return texture
}

// An account that never visited is registered with its avatar.
func TestAccountAvatarOfANewAccount(t *testing.T) {
	db := testDB(t)
	events := make(chan string, 10)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db,
		OnUserChanged:    func(id uint32, name string) { events <- fmt.Sprint("user ", id, " ", name) },
		OnPictureChanged: func(id uint32, picture bool) { events <- fmt.Sprint("picture ", id, " ", picture) },
	}, nil)
	png := pngOf(t, 8, 8)
	if set, err := s.SetAccountAvatar("Alice", "cert-alice", png, false); !set || err != nil {
		t.Fatalf("set: %v %v", set, err)
	}
	r, _ := db.UserByHash("cert-alice")
	if r == nil || r.Name != "Alice" {
		t.Fatalf("registration %+v", r)
	}
	expectEvent(t, events, fmt.Sprint("user ", r.ID, " Alice"))
	expectEvent(t, events, fmt.Sprint("picture ", r.ID, " true"))
	if got := s.savedTexture(t, "cert-alice"); !bytes.Equal(got, png) {
		t.Fatalf("saved %d bytes", len(got))
	}
	// Its first visit: the same registration, with the avatar.
	web := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "Alice", CertHash: "cert-alice"}}, "Alice"))
	if u, _ := s.MumbleClient(web.session); !u.Registered || u.UserID != r.ID {
		t.Fatalf("visit: %+v", u)
	}
	s.mu.Lock()
	texture := s.state.Users[web.session].Texture
	s.mu.Unlock()
	if !bytes.Equal(texture, png) {
		t.Fatal("no avatar on the visit")
	}

	// Kept when only wanted for none.
	if set, err := s.SetAccountAvatar("Alice", "cert-alice", pngOf(t, 9, 9), true); set || err != nil {
		t.Fatalf("replaced: %v %v", set, err)
	}
}

func TestAccountAvatarOfAnOfflineAccount(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	id, _ := db.RegisterUser("bob", "cert-bob")
	db.SetUserContent(id, "my bio", nil)
	png := pngOf(t, 8, 8)
	if set, err := s.SetAccountAvatar("bob", "cert-bob", png, true); !set || err != nil {
		t.Fatalf("set: %v %v", set, err)
	}
	if comment, texture, _ := db.UserContent(id); comment != "my bio" || !bytes.Equal(texture, png) {
		t.Fatalf("saved %q %d bytes", comment, len(texture))
	}
	if set, _ := s.SetAccountAvatar("bob", "cert-bob", pngOf(t, 9, 9), true); set {
		t.Fatal("replaced")
	}
	// Replaced when wanted.
	if set, _ := s.SetAccountAvatar("bob", "cert-bob", pngOf(t, 9, 9), false); !set {
		t.Fatal("not replaced")
	}
}

// Online: its sessions and others see it at once.
func TestAccountAvatarOfAnOnlineAccount(t *testing.T) {
	db := testDB(t)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	web := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "carol", CertHash: "cert-carol"}}, "carol"))
	other := dial(t, addr, "other")
	png := pngOf(t, 8, 8)
	if set, err := s.SetAccountAvatar("carol", "cert-carol", png, true); !set || err != nil {
		t.Fatalf("set: %v %v", set, err)
	}
	expectedHash := blobHash(png)
	for _, c := range []*testClient{web, other} {
		m := c.waitForState(web.session, func(m *pb.UserState) bool {
			return m.Texture != nil || m.TextureHash != nil
		})
		if !bytes.Equal(m.TextureHash, expectedHash) {
			t.Fatalf("avatar hash %x, want %x", m.TextureHash, expectedHash)
		}
		if expectedHash != nil {
			if m.Texture != nil {
				t.Fatalf("hashed avatar was sent inline: %v", m)
			}
			c.send(mumble.TypeRequestBlob, &pb.RequestBlob{SessionTexture: []uint32{web.session}})
			m = c.waitForState(web.session, func(m *pb.UserState) bool { return m.Texture != nil })
		}
		if !bytes.Equal(m.Texture, png) {
			t.Fatalf("texture %d bytes", len(m.Texture))
		}
	}
	eventually(t, func() bool { return bytes.Equal(s.savedTexture(t, "cert-carol"), png) })
	if set, _ := s.SetAccountAvatar("carol", "cert-carol", pngOf(t, 9, 9), true); set {
		t.Fatal("replaced")
	}
}

func TestAccountAvatarIsRefused(t *testing.T) {
	db := testDB(t)
	s, _ := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	// A Mumble user's name: not taken over.
	db.RegisterUser("dave", "mumble-cert")
	if set, err := s.SetAccountAvatar("dave", "cert-dave", pngOf(t, 8, 8), false); set || err != nil {
		t.Fatalf("set: %v %v", set, err)
	}
	if r, _ := db.UserByHash("cert-dave"); r != nil {
		t.Fatal("registered")
	}
	// Not an allowed picture.
	if set, _ := s.SetAccountAvatar("erin", "cert-erin", []byte("<svg/>"), false); set {
		t.Fatal("set a non-picture")
	}
	if r, _ := db.UserByHash("cert-erin"); r != nil {
		t.Fatal("registered")
	}
}
