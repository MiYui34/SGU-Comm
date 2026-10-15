# Voice: DeepFilterNet3 as the strongest noise suppression

## Summary

A third choice next to the browser's filter and RNNoise: DeepFilterNet3
(the @workadventure/noise-suppression build, MIT), much better against
keyboards (about 45 dB less with steady typing in their benchmark).

## Requirements

- Setting "Strongest (DeepFilterNet3)"; it loads (about 11 MB) only when
  chosen.
- It runs before our capture worklet, so the level meter, voice activity,
  encoder and echo test see its result.
- Works under our CSP (web and desktop), with the browser's own noise
  suppression off.
- Too slow (the package's overload signal) or failing to load: back to
  RNNoise, with a note in the settings.

## Acceptance Criteria

- Unit tests for the setting and the fallback rule.
- In the browser (built app, production CSP): it loads, runs and can be
  switched off; the echo test plays it.

## Notes

- Adds 51 to 71 ms of delay. Watch DPDFNet and GTCRN (no browser builds yet).
