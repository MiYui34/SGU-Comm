# Certificate sync regression test decision

The owner approved correcting ShellCheck warnings in the new `tests/deploy/cert-sync.sh` without changing its checks. This is the first review step, recorded as P1 for the path-protection schema. The schema's `execute-phase` authority below identifies the required justification row; it does not claim a separate execute-phase run.

```
path-protection-records@1
kind | paths | phase | date | authority | justification
justification | tests/deploy/cert-sync.sh | P1 | 2026-10-17 | execute-phase | Correct ShellCheck warnings in the newly added certificate regression test without weakening assertions.
approval | tests/deploy/cert-sync.sh | P1 | 2026-10-17 | human-owner | Owner approved this exact test correction and required record during review.
```
