# SUPERUSER_PASSWORD with a $ in deploy/.env

## Summary

Compose reads `$` in deploy/.env as the start of a variable, so a
password like `ab$cd` reaches Kallisti as `ab`. Single quotes keep the
value as written. Document it (install and configuration docs).

## Acceptance Criteria

- Reproduced with `docker compose config`; the docs say to quote.
