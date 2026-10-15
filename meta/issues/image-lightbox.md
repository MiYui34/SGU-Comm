# Image lightbox

## Summary

Images in chat open in a new tab. Show them in a simple lightbox in the
app instead.

## Requirements

- Clicking an image in a message opens it large over the app (dialog).
- Closes with Escape, the close button, or a click outside the image.
- A link to open the original (new tab) stays available.
- Keyboard and screen readers: the image button has a label, focus goes
  back after closing.

## Acceptance Criteria

- Unit test where logic exists; browser check against the built app on
  the Go server (CSP).

## Result

- Plain left click opens the lightbox; middle and modified clicks still
  open a new tab. Closes with Escape, the button, or a click outside the
  image; "Open original" link. Tested against the built app on the Go
  server.
- Headless test tabs are "hidden": Chrome then delays the dialog's close
  event, so the buttons call onClose themselves.
