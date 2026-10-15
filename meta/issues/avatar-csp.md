# Avatars broken: CSP blocks data: images

## Summary

On chat.kallisti.chat avatar pictures show as broken images: the web
app's Content-Security-Policy allows `img-src 'self' blob:`, and avatars
are data: URLs. Local tests ran on Vite, which sends no CSP.

## Acceptance Criteria

- The CSP allows data: images (images only); a test checks it.
- Avatars show on the deployed server.
