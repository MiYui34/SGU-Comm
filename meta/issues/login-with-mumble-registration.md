# Log in with a Mumble registration

## Summary

The web "Log in" tab only knows Kallisti web accounts. SuperUser (and
other Mumble registrations with a password) must use the Guest tab's
server password field, which nobody finds.

## Requirements

- On the built-in server, "Log in" with a name and password that match a
  Mumble registration (SuperUser included) logs in as that registration.
- Web accounts keep working as before; a wrong password still says "wrong
  name or password".

## Acceptance Criteria

- SuperUser logs in through the "Log in" tab and has admin rights.

## Result

- /api/login falls back to Mumble registrations with a password (no web
  account of that name), with the Mumble server's per-IP lockout. No
  session: the client connects with the name and password.
- Tested in the browser (admin rights after login) and with the login
  API on a deployed server.
