# Smoother stream playback

## Summary

Report: a shared tab stutters a lot for a viewer. Today stream frames are
drawn on arrival (no video buffer; sound leads by 0.15 s on one clock), a
lost frame freezes the picture until the next keyframe (viewers ask at
most once a second), and the bitrate is fixed (720p30 ≈ 4 Mbit/s).
Watching is one-way like music: a ~500 ms playout buffer for picture and
sound (kept in sync) smooths jitter. Also: quicker recovery after a drop.
Later, if needed: adaptive bitrate from viewer drops.

## Acceptance Criteria

- Measure first with the perf tool (tests/perf) extended to streams: a
  tab-like moving picture, a throttled/jittery viewer; frame pacing
  (gaps), freezes, drops. Before/after numbers.
- Pure functions for the playout schedule with tests.
