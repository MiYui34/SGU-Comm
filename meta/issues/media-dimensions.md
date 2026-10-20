# Media dimensions: no jumping while loading

## Summary

Uploads of images and videos store width and height (and duration for
videos), so the client reserves the right space (CSS aspect-ratio) before
the media loads: no layout jump in the chat. Go only, no new runtime
dependency: images via internal/imagesize (header only); videos by
reading the container header (MP4/MOV: tkhd/mvhd; WebM/Matroska: EBML
PixelWidth/PixelHeight, Duration) — best effort, unknown formats keep
today's behaviour. Existing uploads get their sizes once (migration that
reads the file headers).

## Acceptance Criteria

- Go tests: dimensions for PNG/JPEG/GIF/WebP and MP4/WebM samples,
  stored on upload, sent with messages, backfilled once.
- Web: media boxes use the size (capped to the max display size) before
  loading; tests for the box size; no jump when scrolled to the bottom.
