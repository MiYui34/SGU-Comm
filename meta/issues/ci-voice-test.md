# CI: voice test in real browsers

## Summary

Voice bugs in Firefox (no terminators) went unnoticed, and Safari could only be tested by hand. A GitHub Actions workflow should check voice in Chrome, Firefox and Safari on every push.

## Requirements

- Start the Kallisti server on the runner.
- Drive each browser with WebDriver: join as a guest, replace the microphone with an oscillator, join a voice channel, make talk spurts with pauses.
- `voicecheck` listens and fails the job on gaps, repeats or missing terminators.
- Chrome and Firefox on Linux, Safari on macOS.

## Acceptance Criteria

- The workflow passes on main for all three browsers, and fails when terminators are missing.
