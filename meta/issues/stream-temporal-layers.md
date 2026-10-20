# Streams: temporal layers for slow viewers

## Summary

A viewer whose line is slower than the stream's bitrate still stutters.
The server never re-encodes. With temporal scalability (WebCodecs VP8
`scalabilityMode: "L1T2"`/`"L1T3"`), frames carry a layer; the server
drops the upper layer(s) for a congested viewer (30 → 15 or 7.5 fps, same
sharpness) and only that viewer is affected. No extra encode, little
streamer cost. OBS (H.264, no layers) keeps today's behaviour.

## Acceptance Criteria

- Streamer encodes with layers where supported (fallback: none); frames
  carry their layer id.
- Server: per viewer, drop upper layers while its queue is congested,
  restore when it drains; tests.
- Measured with tests/perf stream.sh on a link below the bitrate.
