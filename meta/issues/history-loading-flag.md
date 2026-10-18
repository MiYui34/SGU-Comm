# History loading flag clears too early

## Summary

Astra #16: loadingOlder clears on any text event, not only on the page
asked for; duplicate requests follow and a jump can give up. Clear it on
the matching history answer (or an error).

## Acceptance Criteria

- Test: unrelated events keep loadingOlder; the answer clears it.
