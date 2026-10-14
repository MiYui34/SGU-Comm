# Settings: handle localStorage failures and sync between tabs

## Summary

`localStorage.setItem` can throw (private mode, full storage). Settings changed in one tab do not reach other tabs.

## Requirements

- Catch storage errors; settings then live only for the session.
- Listen to the `storage` event to sync settings between tabs.

## Acceptance Criteria

- A storage error does not break the settings dialog.
- A change in one tab shows in another open tab.
