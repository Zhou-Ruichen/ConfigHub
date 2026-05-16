# Technical Direction Research

## Question

What implementation approach best fits ConfigHub as a user-facing hosted configuration hub?

## Context

ConfigHub is not a personal-only helper and not an AI-only tool. It should be deployable on a machine that users and devices can reach. Users should be able to open the hub, inspect templates, copy bootstrap commands, and sync devices.

The product must support:

- hosted web/API service;
- local renderer;
- LAN-accessible configuration server;
- VPS deployment;
- safe client-side apply;
- optional direct remote config reads;
- AI and dotfiles template domains.

## Sources

- Go command documentation: https://pkg.go.dev/cmd/go
- Node.js single executable applications: https://nodejs.org/api/single-executable-applications.html
- Deno compile: https://docs.deno.com/runtime/reference/cli/compile/
- Bun executable compile: https://bun.sh/docs/bundler/executables
- Rust Cargo Book: https://doc.rust-lang.org/cargo/
- MCP transports: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- chezmoi documentation: https://www.chezmoi.io/
- ansible-pull documentation: https://docs.ansible.com/ansible/latest/cli/ansible-pull.html

## Findings

### Naming

Recommended product name: ConfigHub.

Recommended repository slug: `config-hub`.

Recommended command: `confighub`.

This naming matches the broader product because the project covers AI configuration and `.*` files for multiple users/devices, not only one AI runtime setup.

### Language

Recommended MVP language: Go.

Reasons:

- One binary can provide CLI, server, API, and a server-rendered web UI.
- The same binary can run on a VPS, a LAN host, a workstation, or a client machine.
- Go's standard library covers the core needs: HTTP, embedded assets, filesystem operations, checksums, templates, JSON, process execution, and path handling.
- Cross-compilation for macOS and Linux is straightforward.
- Fast startup and low memory usage fit both short-lived CLI commands and a small always-on server.
- Client devices do not need a language runtime or package manager to use the tool.

Alternatives:

- TypeScript/Deno is ergonomic for schema-heavy apps but adds a runtime/toolchain assumption to every client.
- Rust is strong for safety and performance but slower for first-product iteration.
- Python is productive but weaker for portable single-binary distribution.
- Shell should only wrap installation and integration; it should not own diff/apply/rollback.
- Node/Bun can work but provide less operational simplicity than a conventional Go binary for this use case.

### Service Composition

Do not require external services in the MVP.

The core product should provide:

- profile loading;
- template rendering;
- web UI;
- HTTP API;
- bundle metadata;
- bundle download;
- direct template read endpoints;
- client diff/apply/rollback;
- LAN/VPS server mode.

External systems should be optional adapters:

- Secret stores: optional if local references or encrypted files are insufficient.
- Provider gateways: optional if endpoint templates are insufficient.
- MCP gateways: optional if remote endpoint templates or local bridge configuration are insufficient.
- Reverse proxies: optional for TLS/domain deployment convenience.

### Architecture

The custom code should own:

- Profile model.
- Template definition model.
- AI and dotfiles domains.
- Bundle format.
- Renderer.
- Client CLI.
- Server-rendered web UI.
- HTTP API.
- Apply safety policy.
- Lightweight server mode.

The custom code should not own by default:

- Full home-directory sync.
- Full secret-manager semantics.
- Full LLM provider proxying.
- Full MCP server orchestration.
- Database-backed multi-tenant control plane.

## Decision

Build the MVP as a Go single-binary project:

- `cmd/confighub`: CLI entrypoint and `serve` mode.
- `internal/profile`: profile parsing and validation.
- `internal/template`: template discovery and domain rules.
- `internal/render`: profile/templates -> immutable bundle.
- `internal/bundle`: manifest, checksums, bundle read/write.
- `internal/apply`: diff, backup, atomic write, rollback.
- `internal/server`: HTTP API.
- `internal/web`: server-rendered pages and embedded static assets.
- `internal/secret`: local references first, adapters later.
- `internal/domain`: AI and dotfiles domain handlers.

The first implementation slice should be local-only render/apply, followed by hosted read-only web/API, then pull/apply from the hub.
