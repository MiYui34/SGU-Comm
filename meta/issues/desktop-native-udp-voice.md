# Desktop: native UDP voice

## Summary

Part of [Desktop app (Tauri)](desktop-app-tauri.md). Voice over UDP has lower latency than the TCP tunnel on lossy networks. Mumble encrypts UDP voice with OCB-AES128 (keys from CryptSetup).

## Requirements

- OCB-AES128 as Mumble uses it, with nonce handling and resync (CryptSetup).
- UDP ping, fall back to the TCP tunnel when UDP does not work.

## Acceptance Criteria

- Voice goes over UDP against Murmur, and falls back to TCP when UDP is blocked.

## Result

- Done. Tests against the real Murmur: Murmur decrypts the bridge's UDP
  voice (`from_client.good`), and a second client hears voice over UDP.
  Without UDP answers, voice stays on TCP.
- Later: the UDP format follows the server version only. Murmur also needs a
  1.5 client; the web client sends 1.5, so this is not a problem now.
