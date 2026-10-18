# Admin view of all connections

## Summary

In the server settings, for admins: a table of connected users with ping,
late or lost voice, dropped frames and transport, updating every few
seconds, worst first. The server logs one line when a connection stays
bad for a while (and when it is good again), so problems can be found
after the fact.

## Acceptance Criteria

- Server tests: admins only; the log line with hysteresis (no flood).
- Web: the table, sorted worst first.
- Needs connection-stats-server.md.
