# Audio limiter for user volume above 100 %

## Summary

User volume goes up to 200 %. Loud speakers can clip at high gain. Unmuting a user in the middle of a talk spurt can give a short glitch, because their decoder skipped packets.

## Requirements

- A limiter (for example a DynamicsCompressorNode) on the playback output.
- Optional: reset a user's decoder when they are unmuted.

## Acceptance Criteria

- A full-scale test tone at 200 % volume does not clip at the output.
