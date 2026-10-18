# Screen share left running after a connection drop

## Summary

Astra #2: if the socket closes while the screen picker is open, the
capture and encoder start anyway and wait for ever; the share button
stops working until reload. Check after each await; stop stale tracks.

## Acceptance Criteria

- Test or careful reasoning with a manual check.
