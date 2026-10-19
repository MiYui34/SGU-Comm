package mumbled

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"kallisti.chat/server/internal/names"
)

// Avatars serves registered users' avatars at /api/avatars/<name>, so web
// clients can show them for users who are offline, and for online ones
// instead of a RequestBlob on the voice connection (with ?v=<SHA-1> from
// UserState: cached for good). Only real images (by their bytes) go out;
// anything else is 404.
func (s *Server) Avatars() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		texture, err := s.avatarOf(strings.TrimPrefix(r.URL.Path, "/api/avatars/"))
		if err != nil {
			slog.Error("reading an avatar", "err", err)
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		typ := imageType(texture)
		if typ == "" {
			NoAvatars.ServeHTTP(w, r)
			return
		}
		sum := sha1.Sum(texture)
		hash := hex.EncodeToString(sum[:])
		h.Set("ETag", `"`+hash+`"`)
		if r.URL.Query().Get("v") == hash {
			h.Set("Cache-Control", "max-age=31536000, immutable") // a new picture has a new hash
		} else {
			h.Set("Cache-Control", "no-cache") // revalidate: users change pictures
		}
		h.Set("Content-Type", typ)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(texture))
	})
}

// avatarOf is a registered user's avatar: the live one while online (it
// is saved a moment after the UserState with its hash goes out), else the
// saved one.
func (s *Server) avatarOf(name string) ([]byte, error) {
	key := names.Key(name)
	s.mu.Lock()
	for _, u := range s.state.Users {
		if u.Registered && names.Key(u.Name) == key {
			texture := u.Texture // never changed in place
			s.mu.Unlock()
			return texture, nil
		}
	}
	s.mu.Unlock()
	if s.cfg.DB == nil {
		return nil, nil
	}
	return s.cfg.DB.textureByName(name)
}

// NoAvatars is the answer for a user without one (clients ask again later).
var NoAvatars = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=300")
	http.NotFound(w, r)
})
