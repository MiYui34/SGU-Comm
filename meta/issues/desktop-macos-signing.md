# Desktop: sign and notarize the macOS build

## Summary

The macOS build on GitHub is signed with the Developer ID certificate and
notarized (App Store Connect API key), so macOS opens it without a
warning. Secrets: APPLE_CERTIFICATE (no password), APPLE_SIGNING_IDENTITY,
APPLE_API_ISSUER, APPLE_API_KEY, APPLE_API_KEY_P8. Hardened runtime needs
the microphone entitlement.

## Acceptance Criteria

- The CI build is signed and notarized (spctl accepts it); voice works in
  the notarized app.
