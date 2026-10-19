# Join and leave sounds

## Summary

Like Discord: short sounds when someone joins or leaves your voice
channel, when you join or leave, and for mute/unmute and deafen. Soft,
short, our own (no copied Discord sounds). A setting to turn them off (and
a volume); not while deafened; played on the chosen output device. Many
joins at once (server restart) give one sound, not a burst.

## Acceptance Criteria

- Pure functions: which state change gives which sound, the burst limit;
  tests.
- The sounds are small files (or synthesized like the mention ping).
- Manual check: join/leave from a second client.
