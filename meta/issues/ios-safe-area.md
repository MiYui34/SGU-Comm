# iOS PWA: content runs under the status bar

## Summary

In the installed iOS web app the header and drawers reach into the status bar area, which iOS blurs, so the top looks smeared.

## Requirements

- `viewport-fit=cover` and `apple-mobile-web-app-status-bar-style: black-translucent`.
- The app pads top, bottom and sides by the safe-area insets (header, drawers, composer, user panel, sheets, dialogs, toasts); the top strip has the header colour.
- Desktop browsers and Android (insets 0) look exactly as before.

## Acceptance Criteria

- Headless check with emulated insets (CSS override) and without; svelte-check passes; manual check on an iPhone.
