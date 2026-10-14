# mumble-discord

A Mumble client with a Discord-like interface. Web first.

- `packages/protocol`: Mumble protocol (framing, protobuf codec, state)
- `packages/proxy`: WebSocket to Mumble TLS bridge
- `apps/web`: Svelte web client

```sh
pnpm install
pnpm test
```
