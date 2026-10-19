# Streaming from OBS (WHIP)

## Summary

Let users stream from OBS (or any WHIP encoder) into their voice channel,
as on Twitch: in OBS, Stream → service "WHIP", with a server URL and a
personal stream key from Kallisti. The stream shows as LIVE in the
user's current voice channel, like a screen share. Discord has no such
input (its users share an OBS projector window or the virtual camera).

## Design

- Ingest: WHIP (WebRTC-HTTP Ingestion Protocol, built into OBS 30+) at
  e.g. `POST /api/whip`, Bearer stream key. The server takes the WebRTC
  session (pion), one video and one audio track.
- No re-encoding: OBS sends H.264 and Opus. Video frames go to viewers as
  `avc1` frames over the existing `/api/stream` path; Opus goes the
  existing stream-sound path (Mumble clients hear it too). Keyframes on
  request (PLI) for late viewers.
- Stream keys: per registered user, shown and regenerated in Settings;
  stored hashed. Streaming needs Speak in the user's current channel and
  the user to be connected (the stream belongs to their voice session);
  the stream ends when they leave or OBS stops (WHIP DELETE).
- Transport: WebRTC needs a UDP port (configurable, opened in the
  compose file and firewall docs); ICE-TCP or a single muxed port as a
  fallback if feasible. No STUN/TURN needed for a public server.
- Limits: the existing stream limits (bitrate, resolution, number of
  streams) apply; refuse other codecs with a clear error.

## Acceptance Criteria

- Server tests: key auth (hashed, regenerate), Speak needed, H.264/Opus
  forwarded without re-encoding, ends on leave and on WHIP DELETE,
  limits.
- Docs: OBS setup steps; the new port.
- Manual check: OBS → WHIP → watched in the web client and heard in a
  Mumble client.

## Notes

- Viewers without H.264 decoding (some Linux Chromium builds) cannot
  watch these streams; say so in the viewer.
- RTMP would also work but brings AAC (transcode to Opus: CPU, code);
  WHIP is the cleaner path.
