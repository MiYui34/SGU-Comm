# Mumble server: Mumble 1.4 clients

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- The legacy UDP format (varint header) for clients before 1.5, with
  conversion between the formats when 1.4 and 1.5 clients talk.

## Acceptance Criteria

- A 1.4 client and a 1.5 client hear each other.

## Result

- Done: legacy UDP format (PacketDataStream varints, little-endian
  positions), conversion per receiver, legacy pings, a warning for clients
  without Opus. Positions go only to receivers in the same game (plugin
  context), as in Murmur.
- Official client test with Mumble 1.3.4 (Debian bookworm image): its
  voice reaches a 1.5 client, its UDP decrypts, and it shows a 1.5 client
  talking.
