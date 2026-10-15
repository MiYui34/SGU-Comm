# Show pictures of offline users

## Summary

The client finds pictures only for online users, so old messages of users
who left show initials. The server keeps registered users' pictures.

## Requirements

- `GET /api/avatars/<name>`: the saved picture of a registered user (PNG,
  JPEG, GIF or WebP only), else 404. Cache headers.
- Verified messages of offline authors use it; a failed load shows initials.

## Acceptance Criteria

- Server tests for the endpoint (found, unknown name, not an image).
- In the browser, a registered user's picture stays on their messages after
  they leave.

## Notes

- Only with the built-in server; with an external Murmur the endpoint is 404.
- The desktop app's CSP allows no server images: it keeps initials there.
