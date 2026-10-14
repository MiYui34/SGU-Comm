# Desktop: native UDP voice

## Summary

Part of [Desktop app (Tauri)](desktop-app-tauri.md). Voice over UDP has lower latency than the TCP tunnel on lossy networks. Mumble encrypts UDP voice with OCB-AES128 (keys from CryptSetup).

## Requirements

- OCB-AES128 as Mumble uses it, with nonce handling and resync (CryptSetup).
- UDP ping, fall back to the TCP tunnel when UDP does not work.

## Acceptance Criteria

- Voice goes over UDP against Murmur, and falls back to TCP when UDP is blocked.
