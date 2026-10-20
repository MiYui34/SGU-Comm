# Avatar flags recomputed; size limit for chat images

## Summary

1. The "has a picture" flag of registered users predates the 1024 px check
   (review fix #9): recompute it once at startup (migration), so old
   oversized avatars give no 404 that clients show as a broken picture.
2. Images inside chat messages (server/internal/mumbled/images.go, inline
   images from Mumble clients; uploads?) are only type-checked: add a
   dimension limit (decode the header only), as for avatars.

## Acceptance Criteria

- Go tests: migration recomputes the flags (oversized → off); oversized
  chat images refused/stripped with a clear reason.
