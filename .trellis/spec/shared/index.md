# Shared Development Guidelines

These guidelines apply to all ConfigHub code.

## Documentation Files

| File | Description | When to Read |
| --- | --- | --- |
| [go.md](./go.md) | Go style, package boundaries, errors, tests | Always before coding |
| [bundle-contract.md](./bundle-contract.md) | Profile, template, bundle, manifest, checksum, lifecycle contracts | Rendering, API, pull, apply |
| [config-safety.md](./config-safety.md) | Forbidden paths, merge strategy, marker blocks, apply flow | Any template or apply change |
| [concurrency.md](./concurrency.md) | Single-writer hub model, atomic-rename rules, lock files | Any code that writes to disk |
| [timestamp.md](./timestamp.md) | Timestamp format | Date/time handling |

## Core Rules

- One binary: `confighub` provides CLI and serve mode.
- File-backed storage is the MVP default; single-writer assumption per [concurrency.md](./concurrency.md).
- No hidden writes: every local file write must flow through manifest verification, diff, backup, atomic write, and apply log.
- No blind sync of home directories.
- No runtime state, private keys, sessions, cookies, caches, local databases, logs, history, GUI state, or `known_hosts`.
- Do not add external services as required dependencies unless a task explicitly changes the MVP boundary.
- Prefer simple Go standard-library solutions before adding dependencies.

## Before Every Commit

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] Temporary-directory apply tests pass for write-path changes
- [ ] Docs/specs updated when behavior contracts change
