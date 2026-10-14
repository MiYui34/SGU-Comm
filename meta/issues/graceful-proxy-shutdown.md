# Graceful proxy shutdown

## Summary

The proxy does not handle SIGTERM/SIGINT, and a listen error (for example EADDRINUSE) only crashes the process.

## Requirements

- Close the bridge cleanly on SIGTERM and SIGINT.
- Clear error message when the port cannot be opened.

## Acceptance Criteria

- SIGTERM closes all connections and exits with code 0.

## Notes

- May be done as part of the server rewrite instead.
