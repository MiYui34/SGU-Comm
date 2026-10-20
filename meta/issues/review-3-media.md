# Review 3: media, streams and CI

## Summary

Resource bounds and permission checks around WHIP, stream thumbnails, previews and recordings, plus packaging and CI gaps.

## Requirements

- M16: Pending WHIP offers have a body-read deadline and a bound on concurrent pending offers.
- M17: Stream thumbnail updates recheck Speak permission like media frames do.
- Stream thumbnails are decoded, must be JPEG, and have a dimension limit.
- Preview pixel checks use coded frame dimensions where the container gives them (or a safe bound).
- A transient ffmpeg execution failure does not mark a file as having no preview forever.
- Recording silence gap fill checks the byte budget before writing.
- The RPM package depends on a libav GStreamer plugin where one exists, or the limitation is documented.
- CI runs the desktop Rust tests.

## Acceptance Criteria

- Regression tests for each server item; Go race tests pass; CI workflow change is valid YAML.
