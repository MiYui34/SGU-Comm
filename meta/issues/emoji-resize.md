# Resize custom emoji when adding them

## Summary

Custom emoji show at most 48×48 (jumbo emoji); 96×96 covers retina
screens. When staff add an emoji, the browser scales the picture down to
fit 96×96 (aspect kept, never scaled up) before the upload, so large
pictures fit the 256 KB limit and load fast. Optionally crop: a square
crop the user can move (and zoom) before saving, with a live preview at
22 and 48 px.

## Acceptance Criteria

- Pure functions for the target size and the crop rectangle, with tests.
- Still images (PNG, JPEG, WebP) are resized to PNG or WebP (keep
  transparency); animated GIF/WebP are kept as they are (canvas would
  drop the animation) and only checked against the limit, with a note.
- The add-emoji form shows the result before saving; the crop is optional.
- Manual check: a large photo and a transparent PNG come out at ≤ 96 px
  and look right in messages and as reactions.
