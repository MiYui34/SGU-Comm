# Bridge keeps connections alive

## Requirements

- The bridge sends Mumble pings upstream for each web connection (built-in
  server and Murmur), so the browser's timers do not matter.
- WebSocket pings to the browser (answered by the browser itself, without
  JavaScript); no answer in time: close, so dead browsers do not stay
  online forever.
- The web client holds a Web Lock while connected (browsers do not freeze
  or discard such tabs).

## Acceptance Criteria

- Bridge tests: pings reach upstream while the client is silent; a browser
  that does not answer WebSocket pings is closed.
