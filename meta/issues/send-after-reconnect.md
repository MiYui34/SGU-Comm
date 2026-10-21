# Sending an attachment across a dropped connection

## Summary

Two gaps in sending a picked attachment on a shaky connection.

## Requirements

- If the upload succeeds but the message is not confirmed by the server (the connection drops before or after sending it), the attachment and caption stay pending with an error, instead of being cleared silently.
- An upload started before a reconnect that finishes afterwards cannot lead to the same attachment being sent twice from the new chat view.

- Idempotency key: the web client sends an optional `nonce` per message (the same for every retry); the server stores it per author and does not create a second message for a known nonce; the nonce is echoed to the sender's connections for confirmation. Without a nonce (Mumble clients, binary text path, old clients) behaviour is unchanged.

## Acceptance Criteria

- Tests for the logic; web tests and svelte-check pass.
