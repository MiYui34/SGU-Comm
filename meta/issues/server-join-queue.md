# Server: a big server state fills a new client's queue

## Summary

`join` queues the full channel and user list before the writer starts. The queue holds 512 frames; more closes the connection. At about 500 channels (fewer with many users), nobody can connect (review R7).

## Requirements

- Start the writer before the state is sent, or send the state with backpressure. Do not only make the queue bigger.

## Acceptance Criteria

- A test: a server with 1000 channels accepts a new client.
