# Backend / Core Guidelines

These guidelines cover the Go CLI, server, renderer, bundle model, apply engine, HTTP API, and supporting state.

## Documentation Files

| File | Description | When to Read |
| --- | --- | --- |
| [service-architecture.md](./service-architecture.md) | Command/server layout and package boundaries | Any core implementation |
| [cli.md](./cli.md) | CLI subcommand, flag, exit code, and output conventions | Any CLI-facing change |
| [http-api.md](./http-api.md) | Web/API route contracts and error behavior | Serve mode, pull, direct reads |
| [apply-engine.md](./apply-engine.md) | Diff, backup, atomic write, rollback | Local file writes |
| [state-directory.md](./state-directory.md) | What lives in `state/`, file modes, retention | Any code that reads or writes `state/` |
| [security.md](./security.md) | Auth, secret redaction, remote access | Hub APIs and templates |
| [quality.md](./quality.md) | Review and validation checklist | Before commit |

## Core Rules

- `confighub serve` and CLI commands share the same renderer, bundle, and apply packages.
- Server mode is a reachable hub for one operator, not a mandatory cloud platform.
- MVP storage is file-backed under the single-writer assumption documented in [shared/concurrency.md](../shared/concurrency.md).
- API handlers do not bypass bundle contracts.
- Pull/apply paths do not trust the server blindly; clients verify manifests, checksums, and (from Slice 5) bundle signatures.
- Direct template reads are allowed only for templates marked as remote-deliverable.
- Token authentication is required for any non-loopback bind from Slice 4 onward.

## Project Layout

The canonical Go module layout lives in [shared/go.md](../shared/go.md). Refer to that file rather than duplicating the tree here.

## Validation

- `go test ./...`
- `go vet ./...`
- Temporary-directory tests for render/apply changes.
