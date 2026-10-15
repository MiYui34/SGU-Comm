# Desktop notifications

## Summary

Browser notifications for mentions and private messages while the tab is
hidden.

## Requirements

- Off until the user turns them on in Settings (permission asked then).
- Click focuses the app and opens the channel or conversation.
- Not while deafened; not for the open channel when the tab is visible.

## Acceptance Criteria

- Unit test for when to notify; browser check.

## Result

- Settings switch (asks permission, stays off when refused, says when the
  browser blocks it). Notifies for live mentions and private messages,
  not while deafened, not for the chat open in a visible tab; a click
  opens the text channel or conversation.
- Tested: unit tests; browser with a stand-in Notification (headless
  Chrome shows none): one notification per mention and per private
  message, none for other messages.
- Limits: Android Chrome needs a service worker for notifications (none
  there now); Safari tabs on iOS have none; a click on a voice chat
  mention only brings the app forward.
