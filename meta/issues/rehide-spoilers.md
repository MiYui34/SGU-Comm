# Hide a shown spoiler again

## Summary

A shown spoiler cannot be hidden again. Text spoilers should hide on a second click; shown image and video spoilers get a small "Hide" button over them.

## Requirements

- A click (or Enter/Space) on a shown text spoiler hides it again. A click on a link inside a shown spoiler opens the link. A click that ends a text selection does not hide it.
- Shown image and video spoilers have a small "Hide" button overlaid at the top right.

## Acceptance Criteria

- Unit tests for the text toggle; web tests and svelte-check pass; a headless browser check of both.
