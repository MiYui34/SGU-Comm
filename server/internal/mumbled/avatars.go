package mumbled

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Avatars serves registered users' saved avatars at /api/avatars/<name>,
// so web clients can show them for users who are offline. Only real
// images (by their bytes) go out; anything else is 404.
func (d *DB) Avatars() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		texture, err := d.textureByName(strings.TrimPrefix(r.URL.Path, "/api/avatars/"))
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
		h.Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
		h.Set("Cache-Control", "no-cache") // revalidate: users change pictures
		h.Set("Content-Type", typ)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(texture))
	})
}

// NoAvatars is the answer for a user without one (clients ask again later).
var NoAvatars = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=300")
	http.NotFound(w, r)
})
