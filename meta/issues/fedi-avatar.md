# Take over the fediverse avatar

## Summary

A user who registers with a fediverse account gets their avatar from the instance; a linked existing account gets it only if it has none.

## Requirements

- On fediverse registration, download the avatar (`avatar_static` if present, else `avatar`) through the guarded fediverse client (size limit), decode with Go (JPEG, PNG, GIF first frame, WebP via golang.org/x/image), check pixel bounds before decoding, square-crop and scale to 128x128, encode JPEG (as the web client's avatarEncode does), and set it as the account's avatar.
- On linking, only if the account has no avatar.
- Failures are not errors: no avatar.

## Acceptance Criteria

- Tests with the fake instance (register sets it, link keeps an existing one, broken/oversized images are skipped); go test -race passes.
