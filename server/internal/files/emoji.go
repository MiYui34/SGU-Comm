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

// readEmojiImage reads and checks an emoji image upload; false: it
// answered with why not.
func readEmojiImage(w http.ResponseWriter, r *http.Request) (ctype string, data []byte, ok bool) {
	http.NewResponseController(w).SetReadDeadline(time.Now().Add(uploadTimeout))
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxEmojiBytes+1))
	switch {
	case err != nil:
		jsonError(w, "cannot read the image", http.StatusBadRequest)
		return "", nil, false
	case len(data) > MaxEmojiBytes:
		jsonError(w, "an emoji image has at most 256 KB", http.StatusRequestEntityTooLarge)
		return "", nil, false
	}
	// The type comes from the content: only images that cannot carry
	// scripts.
	ctype, _, _ = mime.ParseMediaType(http.DetectContentType(data))
	if len(data) == 0 || !slices.Contains(inlineTypes, ctype) {
		jsonError(w, "an emoji is a PNG, GIF, WebP or JPEG image", http.StatusUnsupportedMediaType)
		return "", nil, false
	}
	switch err := imagesize.Inline(bytes.NewReader(data)); {
	case errors.Is(err, imagesize.ErrTooLarge):
		jsonError(w, "the emoji image has too many pixels", http.StatusRequestEntityTooLarge)
		return "", nil, false
	case err != nil:
		jsonError(w, "the emoji image is broken", http.StatusUnsupportedMediaType)
		return "", nil, false
	}
	return ctype, data, true
}

// uploadEmoji stores a custom emoji (POST /api/emoji): the token (from the
// sidecar, for admins and moderators) names it; the body is the image.
func (h *Handler) uploadEmoji(w http.ResponseWriter, r *http.Request) {
	author, name, ok := h.RedeemEmoji(r.Header.Get("X-Upload-Token"))
	if !ok {
		jsonError(w, "invalid or expired upload token", http.StatusUnauthorized)
		return
	}
	ctype, data, ok := readEmojiImage(w, r)
	if !ok {
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

// uploadUserEmoji stores a personal emoji (POST /api/emoji/personal): the
// token (from the sidecar, for registered users) names it and its owner.
func (h *Handler) uploadUserEmoji(w http.ResponseWriter, r *http.Request) {
	user, name, ok := h.RedeemUserEmoji(r.Header.Get("X-Upload-Token"))
	if !ok {
		jsonError(w, "invalid or expired upload token", http.StatusUnauthorized)
		return
	}
	ctype, data, ok := readEmojiImage(w, r)
	if !ok {
		return
	}
	e, err := h.Store.AddUserEmoji(user, name, ctype, data, h.Now())
	switch {
	case errors.Is(err, store.ErrEmojiBlocked):
		jsonError(w, sidecar.UserEmojiError(err), http.StatusForbidden)
		return
	case errors.Is(err, store.ErrTooManyNewEmoji):
		jsonError(w, sidecar.UserEmojiError(err), http.StatusTooManyRequests)
		return
	case err != nil:
		jsonError(w, sidecar.UserEmojiError(err), http.StatusConflict)
		return
	}
	if h.OnUserEmoji != nil {
		h.OnUserEmoji(user)
	}
	writeJSON(w, map[string]any{"name": e.Name, "id": e.ImageID, "url": sidecar.UserEmojiURL(e.ImageID)})
}

// serveEmoji serves a custom emoji's image (GET /api/emoji/{id}), or a
// personal one's (GET /api/emoji/personal/{id}).
func (h *Handler) serveEmoji(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/emoji/")
	personal := strings.HasPrefix(rest, "personal/")
	id, err := strconv.ParseInt(strings.TrimPrefix(rest, "personal/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var ctype string
	var data []byte
	var at time.Time
	if personal {
		var e store.EmojiImage
		e, err = h.Store.EmojiImage(id)
		ctype, data, at = e.Type, e.Data, e.CreatedAt
	} else {
		var e store.Emoji
		e, err = h.Store.EmojiByID(id)
		ctype, data, at = e.Type, e.Data, e.CreatedAt
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", ctype)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// Ids are never used again and an emoji's image never changes. A
	// personal one can be removed by moderators: clients then show text
	// (they get the removed ids), but shared caches would keep serving it.
	cache := "public"
	if personal {
		cache = "private"
	}
	hd.Set("Cache-Control", cache+", max-age=31536000, immutable")
	http.ServeContent(w, r, "", at, bytes.NewReader(data))
}
