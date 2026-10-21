# Phone: a simpler top bar with a channel sheet

## Summary

On phones the chat header holds the menu, channel name, topic, search box, threads, pins and members: too much. Do as Discord's phone app.

## Requirements

- Phones (max-width 700px): header shows menu button, channel name, a search icon and the members icon only.
- Tapping the channel name opens a channel sheet: topic, Pins, Threads, notification settings (mute and level), copy link. Reuse the message sheet pattern.
- The search icon opens a full-width search field over the header with a back button; results as today.
- DMs and voice views get the same treatment where their header items differ.
- Desktop and tablet unchanged.

## Acceptance Criteria

- Logic tests where any; svelte-check and web tests pass; headless check at 390 px: header items, channel sheet actions, search open/close.
