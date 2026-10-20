# Fixes from the third whole-code review

## Summary

The third review (GPT-6 Astra, 2026-10-20) found one server crash and a set of race, permission and client-state gaps. Fix all of them, then get a second review. Work is split into four subissues by area.

## Requirements

- [Voice sessions and bans](review-3-voice-sessions.md): H1, M3, M4, M5, M6.
- [Server races and authorization](review-3-server-races.md): M1, M2, M7, M13.
- [Client state](review-3-client-state.md): M8, M9, M10, M11, M12, M14, M15.
- [Media, streams and CI](review-3-media.md): M16, M17 and the Low findings.

## Acceptance Criteria

- All subissues are archived.
- A follow-up review finds none of these open.

## Notes

- Finding ids refer to the review report of 2026-10-20.
