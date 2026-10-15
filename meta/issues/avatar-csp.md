# Avatars broken: CSP blocks data: images

## Summary

On chat.kallisti.chat avatar pictures show as broken images: the web
app's Content-Security-Policy allows `img-src 'self' blob:`, and avatars
are data: URLs. Local tests ran on Vite, which sends no CSP.

## Acceptance Criteria

- The CSP allows data: images (images only); a test checks it.
- Avatars show on the deployed server.

## Result

- CSP `img-src 'self' blob: data:`; site test checks it. Tested with the
  built web app on the Go server (real CSP): pictures load in the user
  panel, member list and chat. Deployed; the live header has data:.
