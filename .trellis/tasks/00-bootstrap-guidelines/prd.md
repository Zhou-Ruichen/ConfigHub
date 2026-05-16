# Bootstrap Guidelines and ConfigHub Direction

## Goal

Define **ConfigHub** as a self-hosted configuration hub for a **single operator with multiple machines**, then make Trellis useful for the next implementation slices by locking in requirements, architecture, task breakdown, and quality expectations.

## Target User

ConfigHub MVP targets **one operator who runs the hub and owns every client machine that consumes it**.

- The operator is the only writer of `profiles/`, `templates/`, and bundles on the hub.
- Client machines may include laptops, workstations, VPSs, and LAN hosts owned by the same operator.
- "Multi-user" features (RBAC, per-user audit, organization-level isolation) are explicitly out of MVP scope.
- "Multi-device" support is in scope: device-scoped profiles, per-device bundles, and a hub reachable from several machines.
- The threat model assumes the operator trusts their own LAN/VPS but does not trust public networks. Therefore token-based access is required for any non-loopback bind, and HTTPS is required as soon as the hub is reachable outside trusted home/office LAN.

## Background / Known Context

- The repository started as `ai-config-hub`, but the product scope is now broader than AI configuration.
- The product is not only one personal config dump; it is a hub deployable on a reachable machine that the same operator pulls from across devices.
- Users should be able to open the hub URL, inspect available profiles/templates, copy bootstrap commands, and sync a device.
- Client machines should either sync rendered config bundles locally or directly read remote config endpoints when a tool supports that.
- Template domains cover AI config and selected `.*` / dotfiles config.
- Dotfiles remains a useful reference for boundaries: shared/template/local/runtime separation, no private keys, no runtime state, no blind session sync.
- Infisical, LiteLLM, Docker MCP Gateway, and similar tools are references or optional adapters, not mandatory dependencies.
- This work remains merged into `00-bootstrap-guidelines`.

## Naming Decision (Locked)

- Product name: **ConfigHub**.
- Repository slug: **`config-hub`** (and the local directory will be renamed from `ai-config-hub` to `config-hub` as the last step of this task).
- Command name: **`confighub`**.
- Go module path placeholder until the repo is published: **`github.com/ruichen/config-hub`** (revisit before the first public release).

Rationale:

- "ConfigHub" is broader than AI runtime config.
- It naturally includes `.*` files and environment templates.
- It communicates the hosted product model: one reachable hub for configuration.
- It is clearer for the operator and any future collaborators than `ai-config-hub` once dotfiles templates are included.

## Requirements

### Functional

- Provide a deployable service on a reachable machine via one Go binary.
- Provide a small server-rendered web UI for browsing profiles/templates and copying setup commands.
- Provide an HTTP API for metadata, bundle downloads, and direct template reads.
- Provide a CLI for `init`, `pull`, `diff`, `apply`, `rollback`, `doctor`, `render`, and `serve`.
- Support local, LAN, and VPS deployment with the same binary.
- Treat AI config and dotfiles config as template domains under the same profile/render/bundle/apply model.
- Support local file sync as the compatibility baseline.
- Support direct remote config endpoints where a tool can consume them.
- Render bundles deterministically given the same inputs (except for explicit timestamp/version metadata).
- Treat manifests as authoritative: every applied file must be listed and checksummed.

### Safety

- Preserve safe boundaries: no private keys, no sessions, no caches, no local runtime databases, no `known_hosts`.
- Reject unknown target paths, symlinks (by default), and templates that include raw secrets unless their policy explicitly allows secret-derived values.
- Every local apply runs through diff -> backup -> atomic write -> apply log -> rollback pointer.
- Clients verify pulled bundle manifests and checksums even when the hub is trusted.

### Security

- Non-loopback `serve` binds require token authentication out of the box (MVP ships with token auth, not as a later hardening step).
- Bundles that may carry secret-derived values (e.g. AI provider credentials) are only deliverable over authenticated transport, and direct remote reads of such templates require explicit per-template opt-in plus an authenticated requester.
- HTTPS terminates either through a reverse proxy (recommended) or through built-in TLS once non-LAN exposure begins. The README must call this out before showing any non-loopback bind example.
- Apply logs, audit logs, and API metadata redact secret-derived values.

### Operational

- Avoid mandatory databases and mandatory sidecar services in the MVP.
- File-backed storage is the default; concurrency relies on a single-writer (operator) assumption with render-to-tmp + atomic rename for hub artifacts.
- All template lifecycle events (add / rename / remove) flow through manifest fields, so clients can converge without manual cleanup.
- Persist the technical direction in Trellis artifacts so future sessions do not rely on chat context.

## Technical Direction

Recommended MVP stack:

- Go for one lightweight CLI/server binary.
- Server-rendered HTML plus embedded static assets for the first UI; minimal embedded JavaScript only for copy-to-clipboard and diff toggles.
- File-backed storage for profiles, templates, bundles, and metadata, under a single-writer assumption.
- Token authentication for non-loopback access from day one.
- Optional external adapters (secret store, provider gateway, MCP gateway, reverse proxy) only after the core hub works.

Go is recommended because the product must run as:

- a hosted web/API service;
- a LAN-accessible configuration server;
- a local renderer;
- a client-side safe apply tool on macOS and Linux.

The MVP should prove safety before broad delivery:

1. Render bundles from example profiles/templates locally.
2. Apply bundles from disk with diff, backup, atomic write, and rollback.
3. Add `confighub serve` with web/API access, including token auth for non-local binds.
4. Add pull, direct-read, and bundle signature verification.

## Acceptance Criteria

- [x] README explains product name, target user, deployment model, architecture, stack, milestones, install, and security defaults.
- [x] Research artifact records technical direction and language tradeoffs.
- [x] Trellis PRD reflects ConfigHub requirements with the single-operator-multi-machine framing.
- [x] Trellis design document covers architecture, concurrency, state directory, secrets, lifecycle, and merge strategy.
- [x] Trellis implementation checklist exists with per-slice acceptance gates and a single non-contradictory auth placement.
- [x] `README.md` is the canonical README filename and removes `secrets.local`, the rename ambiguity, and the unguarded LAN-bind example.
- [x] Project-specific Trellis specs cover backend, frontend, shared, big-question, and the new CLI / concurrency / state docs.
- [x] Repository directory is renamed from `ai-config-hub` to `config-hub` as the last action of this task.

Acceptance criteria for the scaffold itself (Go module, `cmd/confighub`, internal packages, fixture profiles) move to the follow-up task `01-go-scaffold` and are **not** part of `00-bootstrap-guidelines`.

## Slice Outcomes (high-level)

Detailed checklists live in `implement.md`. This PRD lists outcomes only so the two documents do not drift.

- **Slice 0 (this task)**: ConfigHub direction, naming decision, specs, README, repo rename.
- **Slice 1**: Go module, command skeleton, internal package boundaries, fixture profiles/templates/bundles, contract tests.
- **Slice 2**: `confighub render` produces manifest + checksums for fixture inputs; forbidden paths and policy-disallowed secrets are rejected.
- **Slice 3**: `confighub diff` / `apply` / `rollback` operate end-to-end against a temp directory; apply log + rollback pointer exist.
- **Slice 4**: `confighub serve` exposes status / profile / template / bundle pages and API routes, with token auth required for non-loopback binds.
- **Slice 5**: `confighub init` / `pull` / `apply` from a hub, including bundle signature verification before any broader rollout.
- **Slice 6**: AI domain templates (Codex, Claude, OpenCode, provider endpoints, MCP endpoints) and `doctor:tools`.
- **Slice 7**: Dotfiles domain templates with documented fragment include strategy and forbidden-path enforcement.
- **Slice 8**: Hardening (audit log retention, restore tests, release builds, optional adapters).

## Out of Scope

- Multi-user accounts, RBAC, or organization-level isolation.
- Blind full-home-directory sync.
- Replacing Dotfiles as a source repository for shared environment files.
- Syncing sessions, cookies, history, caches, databases, private keys, or `known_hosts`.
- Requiring Infisical, LiteLLM, Docker MCP Gateway, Docker Compose, Kubernetes, or a database for the MVP.
- Building a full provider gateway before endpoint templates fail a concrete requirement.
- Building a full MCP gateway before endpoint templates or local bridge config fail a concrete requirement.
- Windows support (not refused; just not validated in MVP).

## Research References

- `.trellis/tasks/00-bootstrap-guidelines/research/technical-direction.md`
- `~/Dotfiles/README.md`
- `README.md`
