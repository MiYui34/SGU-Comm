# Markdown formatting in messages

## Summary

Render a small Markdown subset in messages: bold, italic, strikethrough,
inline code, code blocks, quotes.

## Requirements

- Rendering in the client only, on already escaped text: no HTML from users.
- Links, mentions and the rest keep working; nothing inside code is
  formatted.

## Acceptance Criteria

- Unit tests for each form, nesting limits, code staying literal, and that
  HTML stays escaped.

## Notes

- Mumble clients see the plain text.
