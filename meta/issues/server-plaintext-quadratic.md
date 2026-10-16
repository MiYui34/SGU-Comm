# Server: HTML stripping is quadratic and runs under the server lock

## Summary

`plainText` (`mumbled/messages.go`) strips tags until nothing changes. Input like `"<"*N + "b>"*N` needs about N passes. A guest comment of about 36 KB blocks `Server.mu` for seconds (review R1, High).

## Requirements

- Strip in one pass, so the result is never read again as new markup.
- Normalize outside `Server.mu`; check and apply inside.
- Also: `ReservedName` queries the database while `Server.mu` is held (registration). Move it out.

## Acceptance Criteria

- A test with adversarial input finishes in linear time.
- The existing sanitizer tests still pass.
