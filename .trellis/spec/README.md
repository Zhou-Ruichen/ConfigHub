# ConfigHub Development Guidelines

Production development guidelines for ConfigHub, a lightweight self-hosted configuration hub for a single operator with multiple machines.

## Product Boundary

ConfigHub is a hosted configuration template and bundle service. It supports AI configuration and selected `.*` / dotfiles templates through the same profile, render, bundle, and apply model.

It must not become blind home-directory sync. Runtime state, private keys, sessions, cookies, caches, local databases, logs, history, GUI state, and `known_hosts` are forbidden unless a future spec explicitly allows a narrow exception.

Multi-user accounts, RBAC, and organization-level isolation are out of MVP scope.

## Structure

### [Backend / Core](./backend/index.md)

Go service, CLI, API, renderer, bundle, apply-engine, state directory, and CLI ergonomics guidelines.

### [Frontend / Web UI](./frontend/index.md)

Server-rendered web UI guidelines for the first product UI.

### [Shared](./shared/index.md)

Cross-cutting rules for Go code, bundle contracts, safety policy, concurrency, timestamps, and documentation.

### [Common Design Questions](./big-question/index.md)

Thinking guides for configuration distribution, template boundaries, secret handling, and system constraints.

### [Guides](./guides/index.md)

Pointers to the project-specific guides above. No generic-template legacy content.

## Tech Stack

- **Language**: Go.
- **Binary**: one `confighub` binary for CLI and serve mode.
- **Storage**: file-backed profiles, templates, bundles, and metadata under a single-writer assumption.
- **Server**: Go HTTP server with token auth for non-loopback binds.
- **UI**: server-rendered HTML with embedded static assets; minimal JavaScript only for copy-to-clipboard and diff toggle.
- **Deployment**: local, LAN, or VPS with the same binary; TLS via reverse proxy recommended for non-LAN exposure.
- **Client**: pull, diff, apply, rollback, and doctor commands.

## Before Coding

1. Read the active task artifacts.
2. Read [shared/index.md](./shared/index.md).
3. Read [backend/index.md](./backend/index.md) for CLI/server/apply work.
4. Read [frontend/index.md](./frontend/index.md) for web UI work.
5. Read [big-question/index.md](./big-question/index.md) before changing bundle, template, target path, secret handling, or apply behavior.

## Validation Baseline

- `go test ./...`
- `go vet ./...`
- Temporary-directory render/diff/apply tests before any real home-directory write support.
