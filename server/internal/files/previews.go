package files

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"kallisti.chat/server/internal/imagesize"
	"kallisti.chat/server/internal/mediasize"
	"kallisti.chat/server/internal/store"
	"kallisti.chat/server/internal/thumbs"
)

// Previews (with ffmpeg, see thumbs): a video's frame as its poster, a
// large picture made smaller for the chat (the original is for the
// lightbox and downloads). Made after the upload is stored, never in its
// request; kept next to it as <key>.preview and served at its URL with
// ?preview, so the same key gives access. A failed or useless preview is
// not tried again (store.NoPreview).

const (
	thumbSide   = 480  // a video's frame, the longer side
	previewSide = 1000 // a picture's preview
	// Pictures with a longer side or more bytes get a preview.
	bigSide  = 1600
	bigBytes = 1 << 20
)

func previewPath(dir, key string) string { return filepath.Join(dir, key+".preview") }

// previewJob is the job for an upload's preview, if it gets one. GIFs keep
// their animation (none); PNG and WebP only in a format with transparency.
// Nothing with more pixels than inline pictures may have (ffmpeg's memory).
func previewJob(u store.Upload, dir string, f thumbs.Format) (thumbs.Job, bool) {
	j := thumbs.Job{Src: filepath.Join(dir, u.Key), Dst: previewPath(dir, u.Key), Type: u.Type}
	pixels := u.Width * u.Height
	switch {
	case !thumbs.Supported(u.Type) || pixels > imagesize.MaxPixels:
		return j, false
	case strings.HasPrefix(u.Type, "video/"):
		j.Max, j.Seek = thumbSide, u.Duration >= 2000
		return j, true
	}
	big := max(u.Width, u.Height) > bigSide || u.Size > bigBytes
	j.Max = previewSide
	return j, pixels > 0 && big && (f.Alpha || u.Type == "image/jpeg")
}

// job is an upload's preview job (with a JPEG's Exif orientation).
func (h *Handler) job(u store.Upload) (thumbs.Job, bool) {
	if h.Previews == nil || !plainKey(u.Key) {
		return thumbs.Job{}, false
	}
	j, ok := previewJob(u, h.Dir, h.Previews.Format())
	if ok && u.Type == "image/jpeg" {
		if f, err := os.Open(j.Src); err == nil {
			st, _ := f.Stat()
			j.Orientation = mediasize.ExifOrientation(f, st.Size())
			f.Close()
		}
	}
	return j, ok
}

// preview queues the preview of a new upload, if it gets one. A full
// queue drops it (MakeMissingPreviews makes it at the next start).
func (h *Handler) preview(u store.Upload) {
	j, ok := h.job(u)
	if ok && !h.Previews.Enqueue(j, func(ctype string, err error) { h.previewDone(u, ctype, err) }) {
		slog.Warn("preview queue full", "key", u.Key)
	}
}

// previewDone keeps a preview's type, or NoPreview if the file gives none
// (thumbs.ErrFailed; other errors leave it to the next start). A
// picture's preview must be smaller than the picture; one of an upload
// deleted meanwhile is removed.
func (h *Handler) previewDone(u store.Upload, ctype string, err error) {
	path := previewPath(h.Dir, u.Key)
	if st, serr := os.Stat(path); err == nil && strings.HasPrefix(u.Type, "image/") && (serr != nil || st.Size() >= u.Size) {
		err = fmt.Errorf("%w: not smaller than the picture", thumbs.ErrFailed)
	}
	if err != nil {
		os.Remove(path)
		if !errors.Is(err, thumbs.ErrFailed) {
			slog.Warn("preview not made, tried again at the next start", "key", u.Key, "err", err)
			return
		}
		slog.Info("no preview", "key", u.Key, "type", u.Type, "err", err)
		ctype = store.NoPreview
	}
	found, err := h.Store.SetUploadPreview(u.Key, ctype)
	if err != nil || !found {
		if err != nil {
			slog.Error("store preview", "key", u.Key, "err", err)
		}
		os.Remove(path)
		return
	}
	if ctype != store.NoPreview && h.OnPreview != nil {
		h.OnPreview(u.Key)
	}
}

// MakeMissingPreviews makes the previews attached uploads lack (from
// before previews, or dropped from a full queue), one at a time, the
// newest first. Run it in the background at start.
func (h *Handler) MakeMissingPreviews(ctx context.Context) {
	if h.Previews == nil {
		return
	}
	todo, err := h.Store.UploadsWithoutPreview()
	if err != nil {
		slog.Error("uploads without preview", "err", err)
		return
	}
	for _, u := range todo {
		j, ok := h.job(u)
		if !ok {
			continue
		}
		ctype, err := h.Previews.Make(ctx, j)
		if ctx.Err() != nil {
			return
		}
		h.previewDone(u, ctype, err)
	}
}
