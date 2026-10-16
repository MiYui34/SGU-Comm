# Direct messages: store and sidecar

## Requirements

- Store: a conversation per user pair (hidden chat row); find or make it.
- Sidecar: a "dm" request (user id) answers the chat id; history, send (with uploads and replies), edit, delete, react, read marks work there for the two users only; others get errors.
- New messages go to the two users' connections; the recipient learns of a new conversation with a "dm" event (chat id, peer id and name).
- Ready lists the user's conversations with unread counts.
- Search does not show other people's conversations.

## Acceptance Criteria

- Sidecar tests: delivery to both, refusal for a third user, ready listing, search privacy.
