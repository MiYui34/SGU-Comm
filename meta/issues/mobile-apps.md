# Mobile apps (Android and iOS) with background calls

## Summary

Web pages and PWAs get frozen in the background on mobile (calls drop
when the screen goes dark or on app switches). Native apps keep calls
alive. Tauri 2 builds for Android and iOS; the web client runs inside
unchanged (Kallisti mode with session tokens, voice over the WebSocket,
the Opus WASM fallback). Native parts are small Kotlin/Swift plugins.
Not started; in steps.

## Steps

1. Android prototype: Tauri Android; microphone permission for the
   WebView; a foreground service (type microphone) with a persistent
   notification ("In voice: <channel>") while in voice; an APK built by
   CI. Tested by an Android user (calls with the screen off, app
   switches).
2. iOS prototype: Tauri iOS; microphone permission; UIBackgroundModes
   audio with an active AVAudioSession while in voice. Open question to
   answer first: does WKWebView's web process keep running in the
   background with an active microphone? If not, iOS needs a native audio
   part (much more work) — decide then.
3. Later: store releases (Google Play, App Store/TestFlight), native push
   (FCM/APNs; Web Push does not work in the WebView), CallKit on iOS.

## Notes

- CI: Android SDK/NDK and a signing key; iOS needs a macOS runner,
  certificates and provisioning (the Apple developer account exists).
- Global push-to-talk and other desktop plugins don't apply on mobile.
- Until then: the PWA with the Android media-session keepalive and the
  2-minute server grace; Mumla (Android) and Mumble iOS clients work with
  Kallisti accounts for voice in the background.
