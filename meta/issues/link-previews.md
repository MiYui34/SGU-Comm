# Link previews

## Summary

The server fetches the title, description and image of the first link in a
message and the clients show a card.

## Requirements

- The server fetches, not the readers (their IP stays private).
- Limits: time, size, redirects, only http(s), no private or local
  addresses (also after DNS and redirects).
- The preview image is served by the server (or left out), never loaded
  from the other host.
- Results are cached; a failed fetch shows no card.

## Acceptance Criteria

- Tests: Open Graph and <title> parsing, private address refusal, limits.
- A card shows in the browser for a public page.
