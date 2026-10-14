# Refine the UI towards Discord

## Summary

Make the client look and feel as refined and familiar as Discord, so Discord users feel at home. Compare our screens with current Discord screenshots and close the visible gaps.

## Requirements

- Compare channel list, voice users, chat, member list, user panel and dialogs with Discord screenshots.
- Match layout, spacing, typography, hover states, icons and interaction patterns where it helps familiarity.
- Keep our own name, logo and accent color. Do not copy Discord brand assets (logo, Blurple as brand color, mascot).

## Acceptance Criteria

- Side-by-side screenshots show no large layout or spacing differences in the main view.
- Hover, active and speaking states are present for channels, users and messages.

## Notes

- Familiar layout patterns are fine; brand assets and trade dress are not.
- Client.svelte grows: the direct-message state (dm, readCounts, unread) is a clean candidate for a `lib/directMessages.svelte.ts` factory.
