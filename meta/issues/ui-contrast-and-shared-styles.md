# UI contrast and shared styles

## Summary

Small leftovers from the UI refinement review.

## Requirements

- White text on the teal accent (server icon) is 2.86:1, and the accent status text on the user panel is 4.26:1. Find an accent shade that reaches 4.5:1 for text, or use it only for non-text.
- The text-ellipsis block and the hover-row pattern repeat in ChannelSidebar, MemberList and UserPanel. Move them to shared classes.

## Acceptance Criteria

- All text on accent backgrounds reaches 4.5:1.
- The ellipsis and hover-row styles exist once.
