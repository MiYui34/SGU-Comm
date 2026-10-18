# Stable keys for message rows

## Summary

Astra #14: rows and groups are keyed by object; a reaction makes a new
message object, so the row is built again (media restart, edit focus
lost). Key by message id (and a stable key for Mumble chat lines).

## Acceptance Criteria

- A reaction does not rebuild the row; test where practical.
