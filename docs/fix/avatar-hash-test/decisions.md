# Online avatar test correction

On 2026-10-21, the owner approved updating `server/internal/mumbled/accountavatar_test.go` after `pnpm run test` failed. The 8×8 PNG is 142 bytes; the server correctly announces avatars of at least 128 bytes by hash. Keep the test's two-client delivery and stored-avatar checks, and verify each client can request and receive the full avatar. This is a test-only correction, unrelated to the Helm feature. The `execute-phase` field below is required by the edit guard; it does not claim a separate workflow run or independent review.

```text
path-protection-records@1
kind | paths | phase | date | authority | justification
justification | server/internal/mumbled/accountavatar_test.go | P1 | 2026-10-21 | execute-phase | Owner approved replacing the incorrect inline-only avatar assertion with hash and blob-request assertions for both clients while preserving saved-avatar and only-if-none checks.
```
