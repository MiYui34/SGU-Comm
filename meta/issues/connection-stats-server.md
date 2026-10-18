# Connection stats on the server

## Summary

The built-in server keeps per connection: ping (average and spread, from
the Ping messages of the client and its own), voice frames from the
client (good, late, lost, also for WebSocket voice by sequence number),
frames it could not send (full queue), and text queue overflows. It
answers UserStats with them, as Murmur does (Mumble clients show them in
"User Information"), and fills the Ping reply.

## Acceptance Criteria

- Server tests: counters for UDP and WebSocket voice, dropped frames on
  a full queue, ping averages; UserStats carries them.
- Low cost: counters, no per-packet allocation or logging.
