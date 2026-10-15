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

## Result

- The bridge pings upstream every 10 s for each web connection and pings
  the browser (WebSocket); no answer in 30 s: closed at once. The client
  holds a Web Lock while connected.
- Tested: bridge tests; browser with the page's own pings switched off:
  still connected after 48 s (the server drops silent clients after 30 s).
