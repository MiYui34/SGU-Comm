# Recording a voice channel

## Summary

Moderators and admins record a voice channel. The server already gets
each speaker's Opus stream unmixed: it writes one Ogg Opus file per
speaker without encoding again, with silence where the speaker was quiet,
so the tracks line up for mixing. Everyone in the channel (and who joins
later) sees a red "Recording" indicator; Mumble clients get a channel
message when it starts and stops. When it stops, staff download the files
(a zip) for a limited time.

## Acceptance Criteria

- Server tests: staff only; one track per speaker, aligned in time; the
  indicator state reaches all clients in the channel; a size and time
  limit; files are removed after the download period.
- Web: Record / Stop in the voice channel menu, the indicator, the
  download link.
- Manual check: tracks open in an audio editor and line up.
