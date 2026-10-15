# Avatar size check

## Summary

A small picture file can have very many pixels; each viewer decodes it.

## Requirements

- Pictures larger than 1024x1024 pixels show the initial instead.

## Acceptance Criteria

- Unit test for reading the size (PNG, JPEG, GIF, WebP).

## Result

- imageSize reads PNG, GIF, JPEG and WebP (lossy, lossless, extended)
  headers; larger than 1024 pixels or unknown: the initial shows. Tested
  with built headers and with real images from Chrome's encoders.
