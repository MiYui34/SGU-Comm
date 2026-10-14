# Per-IP connection limit in the proxy

## Summary

All web users reach Murmur from the proxy IP. Murmur's autoban (10 failed attempts in 120 s by default) can ban that IP and lock out everyone.

## Requirements

- Per-IP connection limit in the proxy, in addition to the global limit.

## Acceptance Criteria

- A test shows that connections above the per-IP limit are refused.

## Notes

- Also see the autoban exception in [Production deployment with docker-compose](production-deployment-with-docker-compose.md).
