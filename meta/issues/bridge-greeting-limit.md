# Bridge: no total limit on frames before Authenticate

## Summary

`readHello` (`bridge/bridge.go`) keeps every binary frame until Authenticate. Each message is limited to 8 MiB, but the total is not. A client can fill memory within the 10 s window (review R2, High).

## Requirements

- Set a small total budget for the greeting (for example 64 KiB), and close the connection when it is exceeded.

## Acceptance Criteria

- A test: frames over the budget before Authenticate close the connection.
