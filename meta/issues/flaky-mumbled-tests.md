# Flaky mumbled tests under the full race run

## Summary

In the full `go test -race ./...` run, some mumbled tests time out now and
then while waiting for a frame (5 s): TestBanOptionsFromNewerClients and
TestRolesListsAndUnmakesModerators (in `registered`, waiting for the
user id). Alone they pass every time. Likely load: many test servers at
once under the race detector.

## Acceptance Criteria

- Find the cause (load or a real ordering bug) and fix it: longer waits
  only if it is load, else the ordering.
- The full race run passes 5 times in a row.
