# Voice: better noise suppression

## Summary

RNNoise (WebAssembly) in the capture worklet, as a setting next to the
browser's own filter, and an echo test in the settings to hear the result.

## Requirements

- Setting "Noise suppression": browser (default) or strong (RNNoise). With
  RNNoise the browser's own suppression is off (echo cancellation stays).
- RNNoise runs on the sender before the level meter, voice activity and the
  encoder (48 kHz, two 480-sample frames per 20 ms frame); the module loads
  only when the setting is on.
- Its voice probability helps voice activity (typing alone does not send).
- Echo test in the settings: plays the processed microphone back (what the
  others would hear, before Opus), off when the settings close.

## Acceptance Criteria

- A test: a noisy recording through the denoise step is clearly quieter
  where there is no voice.
- In the browser: the setting switches, the echo test plays.

## Notes

- Later: denoise one other person's voice on the receiver (user menu).
