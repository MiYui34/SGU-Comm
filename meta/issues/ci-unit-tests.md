# Unit tests and type checks in CI

## Summary

Astra #11: CI builds the desktop app and tests voice, but does not run
pnpm test, the type checks or go test. Add a workflow for pushes and pull
requests (with -race for the Go packages).

## Acceptance Criteria

- The workflow runs and passes on GitHub.
