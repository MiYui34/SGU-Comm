# Avatar size check

## Summary

A small picture file can have very many pixels; each viewer decodes it.

## Requirements

- Pictures larger than 1024x1024 pixels show the initial instead.

## Acceptance Criteria

- Unit test for reading the size (PNG, JPEG, GIF, WebP).
