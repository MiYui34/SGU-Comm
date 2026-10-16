# Server: join, move and leave events can be lost

## Summary

The callbacks for joins, moves and leaves (`mumbled/mirrors.go`) are dropped when their queue is full. At shutdown, the worker stops before sessions leave. The sidecar keeps visits and read marks from these events (review architecture note 2).

## Requirements

- Do not drop these events, or reconcile after a drop. Send the remaining events at shutdown.

## Acceptance Criteria

- A test: leave events arrive at shutdown.
