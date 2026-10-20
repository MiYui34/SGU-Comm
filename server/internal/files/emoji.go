package files

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"kallisti.chat/server/internal/imagesize"
	"kallisti.chat/server/internal/sidecar"
	"kallisti.chat/server/internal/store"
)

// MaxEmojiBytes is the largest custom emoji image.
const MaxEmojiBytes = 256 << 10

// uploadEmoji stores a custom emoji (POST /api/emoji): the token (from the
// sidecar, for admins and moderators) names it; the body is the image.
func (h *Handler) uploadEmoji(w http.ResponseWriter, r *http.Request) {
	author, name, ok := h.RedeemEmoji(r.Header.Get("X-Upload-Token"))
	if !ok {
		jsonError(w, "invalid or expired upload token", http.StatusUnauthorized)
		return
	}
	http.NewResponseController(w).SetReadDeadline(time.Now().Add(uploadTimeout))
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxEmojiBytes+1))
	switch {
	case err != nil:
		jsonError(w, "cannot read the image", http.StatusBadRequest)
		return
	case len(data) > MaxEmojiBytes:
		jsonError(w, "an emoji image has at most 256 KB", http.StatusRequestEntityTooLarge)
		return
	}
	// The type comes from the content: only images that cannot carry
	// scripts.
	ctype, _, _ := mime.ParseMediaType(http.DetectContentType(data))
	if len(data) == 0 || !slices.Contains(inlineTypes, ctype) {
		jsonError(w, "an emoji is a PNG, GIF, WebP or JPEG image", http.StatusUnsupportedMediaType)
		return
	}
	if !imagesize.Inline(bytes.NewReader(data)) {
		jsonError(w, "the emoji image is broken or has too many pixels", http.StatusRequestEntityTooLarge)
		return
	}
	e, err := h.Store.AddEmoji(store.Emoji{Name: name, Type: ctype, Data: data, Author: author}, h.Now())
	switch {
	case errors.Is(err, store.ErrNameTaken):
		jsonError(w, "an emoji with this name exists", http.StatusConflict)
		return
	case errors.Is(err, store.ErrTooManyEmoji):
		jsonError(w, "this server has the most emoji it can have", http.StatusConflict)
		return
	case err != nil:
		jsonError(w, "cannot store the emoji", http.StatusInternalServerError)
		return
	}
	if h.OnEmoji != nil {
		h.OnEmoji()
	}
	writeJSON(w, map[string]any{"name": e.Name, "url": sidecar.EmojiURL(e.ID)})
}

// serveEmoji serves a custom emoji's image (GET /api/emoji/{id}).
func (h *Handler) serveEmoji(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/emoji/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	e, err := h.Store.EmojiByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", e.Type)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// Ids are never used again and an emoji's image never changes.
	hd.Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", e.CreatedAt, bytes.NewReader(e.Data))
}
