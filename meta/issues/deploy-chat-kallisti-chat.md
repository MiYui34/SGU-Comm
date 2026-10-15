# Deploy chat.kallisti.chat

## Summary

Run a public Kallisti server on chat.kallisti.chat: the web app over
HTTPS, and the built-in Mumble server for normal Mumble clients.

## Requirements

- Host: the Hetzner server eosrift (eosrift.com, 89.167.43.111). Its
  Caddy already holds ports 80/443: add a site block there, do not run a
  second Caddy.
- DNS: A record chat.kallisti.chat at Porkbun.
- Mumble on port 64738 (TCP and UDP).
- Data in a Docker volume; SuperUser password only on the server.

## Acceptance Criteria

- https://chat.kallisti.chat shows the web app with a valid certificate,
  and a guest can join and talk.
- A normal Mumble client connects to chat.kallisti.chat:64738.
- eosrift keeps working.

## Notes

- The Mumble certificate is self-signed for now: Mumble clients ask once.
