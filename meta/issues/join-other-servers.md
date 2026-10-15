# Join other servers with your account

## Summary

"Join my server": someone sends a Kallisti link, you already have an account
on another Kallisti server, and both servers sit side by side in your app,
as naturally as in Discord. Not started; this is the design.

Guiding rule of the project: it is Mumble, it is easy to use, and when the
two conflict, easy to use wins.

## The flow for the user

1. A friend sends `https://chat.bob.org` (or an invite link).
2. The page offers "Continue as lain (chat.kallisti.chat)" and "Join as
   guest".
3. "Continue as" goes to the home server for a moment: "Let chat.bob.org
   see you as lain?" Allow, and the user is back, logged in. No new
   password. If the name is taken there, "lain@kallisti.chat" is filled in
   and can be changed.
4. The new server appears in the server rail of every server's app; the
   home server keeps the list, so it follows the user to other devices.
5. Switching servers works as in Discord: text and unread marks per server,
   one voice channel at a time across all servers.

## How it works

- Identity: the home server signs "this is lain from chat.kallisti.chat"
  (an OAuth-like redirect between Kallisti servers). The other server makes
  a normal local account linked to it (like guest accounts, but lasting).
  Rights, read marks and reactions already work per local user id.
- One app, many servers: each server is its own site, so cookies cannot
  carry sessions across them. During the join, the app gets an access token
  per server and connects to each with it (allowed origins accordingly).
- Mumble clients: nothing changes; remote users are registered users.

## Decisions (easy over pure)

- The home server holds the identity (not a key only in the browser, which
  is lost with the browser's storage). Users trust their home server, as
  with any service.
- Servers accept any home server by default, with a block list (like guests
  today).
- Plain names where they are free; "name@home" only on a conflict.
- Without an account anywhere, "Join as guest" stays one click.

## Steps (sub-issues when work starts)

1. Several servers in one client (see the issue of that name): the rail, a
   connection per server, one voice channel at a time.
2. Join with your home account: the redirect, the signed confirmation, the
   local account, invite links.
3. The server list on the home account, synced across devices.
4. Later, maybe: iroh in the desktop app, for servers without a domain or
   public IP (iroh does not help the browser case: browsers only use
   relays, and the server is Go).

## Not in scope

- Federation beyond joining (private messages or friend lists across
  servers): that is a much bigger step.
