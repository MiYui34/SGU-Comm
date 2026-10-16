# Streaming: screen share, picture only

## Requirements

- "Share screen" in the voice panel (needs Speak in the channel): the browser's picker (screen, window, tab), then the stream starts.
- The streamer sees a small self-preview with "Stop"; stopping in the browser's own bar stops it too.
- Others in the channel see "LIVE" next to the streamer and can watch (click), inline and full screen; they can stop watching.
- Late viewers get a picture soon (keyframes); slow viewers drop frames, not the connection.
- Ends when the streamer stops, leaves the channel or disconnects.

## Acceptance Criteria

- Server tests: permission, only viewers in the channel, ends on leave, frame dropping for slow viewers.
- Checked in two browsers.
