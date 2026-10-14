// Package files handles uploads (POST /api/upload) and serves stored files
// (GET /files/{key}/{name}).
package files

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/sidecar"
	"kallisti.chat/server/internal/store"
)

// Only these are shown inline: they cannot carry scripts. SVG can, so it
// is a download like everything else.
var inlineTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

type Handler struct {
	Store          *store.Store
	Dir            string
	MaxBytes       int64
	AllowedOrigins []string
	Now            func() time.Time
	// Redeem checks an upload token and returns the uploader's name.
	Redeem func(token string) (string, bool)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/upload":
		h.cors(w, r)
		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			h.upload(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(r.URL.Path, "/files/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		h.serve(w, r)
	default:
		http.NotFound(w, r)
	}
}

// cors lets the web app (possibly on another origin) post uploads.
func (h *Handler) cors(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" || !config.OriginAllowed(h.AllowedOrigins, origin) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Filename")
	w.Header().Add("Vary", "Origin")
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	author, ok := h.Redeem(r.URL.Query().Get("token"))
	if !ok {
		jsonError(w, "invalid or expired upload token", http.StatusUnauthorized)
		return
	}
	key := randomKey()
	dst := filepath.Join(h.Dir, key)
	size, sniffed, err := h.save(dst, r.Body)
	switch {
	case errors.Is(err, errTooLarge):
		jsonError(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	case err != nil:
		slog.Error("save upload", "err", err)
		jsonError(w, "cannot store file", http.StatusInternalServerError)
		return
	case size == 0:
		os.Remove(dst)
		jsonError(w, "empty file", http.StatusBadRequest)
		return
	}
	name, _ := url.QueryUnescape(r.Header.Get("X-Filename"))
	// The stored type comes from the content, not from the client: only a
	// real image becomes an inline image type.
	ctype := "application/octet-stream"
	if slices.Contains(inlineTypes, sniffed) {
		ctype = sniffed
	}
	u, err := h.Store.AddUpload(store.Upload{Key: key, Name: cleanName(name), Type: ctype, Size: size, Author: author}, h.Now())
	if err != nil {
		os.Remove(dst)
		jsonError(w, "cannot store file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"key": u.Key, "name": u.Name, "type": u.Type, "size": u.Size, "url": sidecar.FileURL(u),
	})
}

var errTooLarge = errors.New("too large")

// save writes at most MaxBytes and returns the size and the sniffed type.
func (h *Handler) save(dst string, body io.Reader) (int64, string, error) {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, "", err
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(body, head)
	head = head[:n]
	rest, err := io.Copy(f, io.MultiReader(bytes.NewReader(head), io.LimitReader(body, h.MaxBytes+1-int64(n))))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && rest > h.MaxBytes {
		err = errTooLarge
	}
	if err != nil {
		os.Remove(dst)
		return 0, "", err
	}
	ctype, _, _ := mime.ParseMediaType(http.DetectContentType(head))
	return rest, ctype, nil
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	key, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/files/"), "/")
	u, err := h.Store.UploadByKey(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(h.Dir, u.Key))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	disposition := "attachment"
	ctype := "application/octet-stream"
	if slices.Contains(inlineTypes, u.Type) {
		disposition, ctype = "inline", u.Type
	}
	hd := w.Header()
	hd.Set("Content-Type", ctype)
	hd.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": u.Name}))
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// The key is random and the content never changes.
	hd.Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", u.CreatedAt, f)
}

// cleanName keeps a file name displayable: no path, no control characters,
// at most 200 characters.
func cleanName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	if runes := []rune(name); len(runes) > 200 {
		name = string(runes[:200])
	}
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}

func randomKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
