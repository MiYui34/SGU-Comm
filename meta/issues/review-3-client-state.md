# Review 3: client state

## Summary

Multi-tab and lifecycle gaps in the web client.

## Requirements

- M8: Push opt-out holds across tabs: a sync in another tab does not re-subscribe after an opt-out (shared consent check before committing a subscription).
- M9: A reconnect does not take voice back from another device of the account that joined voice during the outage.
- M10: Only a visible tab advances read marks automatically; a tab that becomes visible catches up.
- M11: Stopping a stream watch before the socket opens cancels the pending watch request.
- M12: A resync after queue overflow restores control state too (threads, follows, read marks), not only message history.
- M14: A combined channel save (name and privacy) reports success only when each part is confirmed by its own matching event.
- M15: Drafts survive opening and closing a thread pane (mobile and desktop).

## Acceptance Criteria

- Unit tests for each item where the logic is testable; svelte-check and web tests pass.
