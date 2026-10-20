# Previews with optional ffmpeg

## Summary

If ffmpeg/ffprobe are installed (the Docker image ships them): video
uploads get a thumbnail (a frame at ~1 s, WebP ~480 px) and duration; the
chat shows the thumbnail with a play button and the duration, the video
loads on click. Large images get a smaller preview (~1000 px) for the
chat; the original loads in the lightbox. Without ffmpeg everything works
as today. ffmpeg runs as a separate process on the uploaded file only
(no network protocols), with a time limit, low priority and a limit on
parallel jobs.

## Acceptance Criteria

- Go tests with a fake ffmpeg (and a real one if present): thumbnail and
  preview made and served; time limit; missing ffmpeg → no previews.
- Docker image includes ffmpeg; docs mention the size and the option.
