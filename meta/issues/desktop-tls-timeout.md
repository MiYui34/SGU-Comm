# Desktop: the TLS handshake to the server has no timeout

## Summary

In `bridge.rs`, the TCP connect has a 10 s timeout, but `tls.connect` has none. A server that stalls the handshake keeps the task alive, also after the window disconnects (review R12).

## Requirements

- One timeout for connect and handshake together. Cancel when the client goes away.

## Acceptance Criteria

- A test with a server that accepts TCP and never answers TLS.
