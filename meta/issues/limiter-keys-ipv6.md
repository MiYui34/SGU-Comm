# One limiter key for all login limits (IPv6 /64)

## Summary

Astra #6: native Mumble login failures and bridge admission count single
addresses; the web login and connection limits count IPv6 /64s. One
shared key helper for all of them.

## Acceptance Criteria

- Tests: failures from two addresses of one /64 count together.
