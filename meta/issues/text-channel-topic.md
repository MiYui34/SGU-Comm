# Text channel topic

## Summary

Text channels have no topic or description.

## Requirements

- Admins and moderators set a topic (plain text, 200 characters) in the
  channel dialog; shown in the chat header; the Mumble mirror gets it as
  its description.

## Acceptance Criteria

- Store, sidecar and web tests.

## Result

- Admins and moderators set a topic (200 characters) in the edit dialog;
  the chat header shows it; the Mumble mirror gets it as description.
  Tested: store, sidecar, mumbled and web tests; browser (SuperUser sets
  it, a guest sees it).
