# Kallisti accounts: password login from Mumble clients

## Summary

A Kallisti account is registered on the built-in Mumble server with a
certificate, not a password. A Mumble client (or the desktop app in
Mumble mode) that sends the account's name and password is refused. The
built-in server should check the password against the account store and
let the user in as that account (failed tries count as failed logins).

## Acceptance Criteria

- Server tests: account name + right password logs in as the account;
  wrong password refused and counted; guest accounts (no password) as
  before; Mumble registrations as before.
