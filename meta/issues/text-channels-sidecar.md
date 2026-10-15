# sidecar: sync mirrors, bridge messages, history

## Requirements

- Sync mirrors at start and after each text channel change; save ids.
- Mumble text in a mirror goes to the text channel (all web users);
  web messages go to the Mumble users in the mirror (uploads as links).
- On entering a mirror: last 20 messages on the first visit in a
  connection, later only those since leaving (at most 20); forget
  sessions that leave the server.
- ready/channels events carry each text channel's Mumble id.

## Acceptance Criteria

- Sidecar tests with a fake voice server.
