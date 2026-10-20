# Limits for moderators' voice subchannels

## Summary

Channel moderators can create permanent voice subchannels without limit.
Like Murmur's channelcountlimit / channelnestinglimit, limit what they
create: at most 50 channels below a moderated channel and at most 5
levels below it. Staff are not limited.

## Acceptance Criteria

- Server tests: the 51st and the 6th level are refused for a channel
  moderator with a clear message (web and Mumble clients); staff pass.
