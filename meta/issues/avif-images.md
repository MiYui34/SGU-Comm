# Show AVIF and JPEG XL images

## Summary

An uploaded `.avif` picture shows as a file card. AVIF is not in the inline types, Go's content sniffing does not know it, and the size checks cannot read its header.

## Requirements

- Detect AVIF (ISO BMFF `ftyp` with brand `avif` or `avis` among major/compatible brands) and treat `image/avif` as an inline type.
- Read width and height from the header (the `ispe` property boxes; for grids take the largest) for the inline image limits (imagesize) and the stored dimensions (mediasize). Malformed or oversized headers are refused like other images (download instead of inline).
- Previews: ffmpeg may make a preview for AVIF like other pictures if it can decode it; no change needed if it already works.
- Clients that cannot show AVIF fall back to the file card (check the existing image error path).

- JPEG XL: most browsers cannot show it, so it stays a download (`image/jxl`, detected by its signatures), but gets an ffmpeg picture preview that the chat shows inline; without ffmpeg the file card stays.

## Acceptance Criteria

- Go tests with real small AVIF files (still and grid) and malformed ones; web tests pass if the client changes.
