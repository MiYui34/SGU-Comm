# Guests cannot set a profile picture

## Summary

Guests keep a picture only until they disconnect (also on an automatic
reconnect), so pictures come and go. Only registered users may set one.

## Requirements

- The built-in server refuses a texture from a guest (clearing is allowed),
  with a reason that tells them to register.
- The settings show guests a hint to register in place of the picture buttons.

## Acceptance Criteria

- A server test: a guest's texture is denied, a registered user's is kept.
- Guests see the hint in the settings.

## Notes

- Murmur allows guest textures; this is a small difference.
- With an external Murmur, only the web UI hides the buttons.
