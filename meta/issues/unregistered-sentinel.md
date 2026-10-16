# Unregistering an online user gives web users one shared user id

## Summary

When an online user is unregistered, the server sends `UserId = 0xffffffff`. The sniffer (`mumble/sniffer.go`) and the TypeScript state take this as a registration. Two such web users then share the id 4294967295 and can edit each other's new messages (review R5).

## Requirements

- Treat `0xffffffff` as "not registered" in Go (sniffer) and in TypeScript (`packages/protocol` state).

## Acceptance Criteria

- A test: register, unregister online, post; the user is a guest and owns only its own messages from this visit.
