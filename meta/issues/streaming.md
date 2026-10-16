# Screen sharing and camera streams (Go Live)

## Summary

Like Discord's Go Live: people in a voice channel share their screen (later their camera) with the others there. Kallisti viewers see picture and sound; desktop Mumble clients hear the stream's sound.

## Design

- Transport: WebCodecs (VideoEncoder/VideoDecoder, Opus for sound) over a WebSocket of its own (`/api/stream`), so video never delays voice and no extra ports, STUN or TURN are needed. The server forwards frames and never re-encodes. A WebRTC SFU (pion) can replace the transport later if quality or latency needs it.
- One stream carries picture and sound with one clock: Kallisti viewers play them in sync.
- For desktop Mumble clients only, the server also forwards the stream's Opus packets as voice of a hidden stream user ("name (stream)") in the channel: their own volume control; its comment links to the web player. Web clients never see that user.
- Permission: Speak in the channel (a listen-only channel blocks streaming, except for its speakers).
- Watching is opt-in (a click), only for people in the same voice channel. A stream ends when the streamer leaves the channel.
- Server limits set by the admin: bitrate, resolution, number of streams.

## Steps (sub-issues)

1. [Screen share, picture only](streaming-screen-share.md)
2. [Several streams: grid and focus views](streaming-views.md)
3. [Stream previews and viewer list](streaming-previews.md)
4. [Stream sound, also for Mumble clients](streaming-sound.md)
5. [Camera and quality while live](streaming-camera.md)

## Out of scope

- Game detection and an in-game overlay (need a native app).
- Streams in direct messages (would need voice calls in DMs first).
- Sharing from mobile browsers (they cannot; watching works).
