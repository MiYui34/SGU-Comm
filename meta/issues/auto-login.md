# Log in at once with a saved session

## Summary

Bug: on start or reload the login screen flashes before the server says
the user is logged in. Show nothing until that is known. And with a
saved session, connect at once: no "Continue as X" (the user can log
out). After a failed connection the screen offers "Try again" and
"Log out" (no loop).

## Acceptance Criteria

- No login screen while the session is checked; a saved session connects
  without a click (web and desktop); errors do not reconnect in a loop.
