# Fixes from the repository reviews

## Summary

The two reviews ([first](repository-review.md), [second](repository-review-6-1-sol.md)) found defects that are worth fixing. Each one has a sub-issue. Findings that are not tracked are listed under Notes.

## Acceptance Criteria

- All sub-issues are done.

## Notes

Not tracked (no realistic impact now, or a known choice):

- Quadratic reaction ordering (R8): needs thousands of users with the same emoji on one message.
- Unbounded chat state in the web client (R9): only for very long sessions. Open an issue if it shows up.
- Marshal each broadcast once, sort sidebar users once, shared wire fixtures: profile first.
- `compose.murmur.yml` with no certificate check: documented for self-signed Murmur servers.
- Inline styles in sanitized HTML: blocked by the CSP.
