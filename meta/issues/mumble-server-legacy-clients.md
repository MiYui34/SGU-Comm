# Mumble server: Mumble 1.4 clients

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- The legacy UDP format (varint header) for clients before 1.5, with
  conversion between the formats when 1.4 and 1.5 clients talk.

## Acceptance Criteria

- A 1.4 client and a 1.5 client hear each other.
