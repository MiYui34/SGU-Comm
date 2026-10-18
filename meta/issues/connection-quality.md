# Connection quality

## Summary

Nobody can see today whether voice works well for everyone. Web and app
users send voice over a WebSocket (TCP), so trouble shows as delay and
late audio, not as lost packets; the server keeps UDP counters (good,
late, lost) for Mumble clients only and no ping times. Sub-issues:

1. connection-info-web.md: each user sees their own connection.
2. connection-stats-server.md: the server measures every connection.
3. connection-admin-view.md: admins see all connections at once.

## Acceptance Criteria

- The sub-issues are done or deliberately dropped.
