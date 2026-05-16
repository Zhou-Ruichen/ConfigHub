# Design: ConfigHub Bootstrap Direction

## Target User

ConfigHub MVP targets a single operator running the hub and owning all client machines that consume from it.

- Single-writer assumption: only the operator edits `profiles/` and `templates/` on the hub.
- Multi-reader: several client machines (laptop, workstation, VPS, LAN host) may pull/read concurrently.
- The operator may trust their own LAN/VPS but does not trust public networks; token auth is required for any non-loopback bind from day one.
- Multi-user accounts, RBAC, and org-level isolation are out of MVP scope.

## System Boundary

ConfigHub manages rendered configuration templates and bundles. It is not a blind home-directory sync tool and it is not limited to AI configuration.

Owned by ConfigHub:

- User/profile/device/role profile definitions.
- Template definitions.
- AI config template rendering.
- Selected dotfiles template rendering.
- Bundle metadata, checksums, signatures, apply logs, backups, and rollback metadata.
- Web UI and HTTP API for browsing, bundle download, and direct template reads.
- Local/LAN/VPS bundle serving.

Not owned by ConfigHub:

- Private keys.
- Login state.
- Browser cookies.
- OAuth refresh tokens.
- CLI session history.
- Local runtime databases.
- `known_hosts`.
- Cache, logs, and GUI state.
- Full package installation or OS provisioning in the MVP.
- Multi-user authorization (single-operator MVP).

## Recommended Architecture

```text
cmd/confighub
  CLI entrypoint and serve mode

internal/profile
  profile parsing and validation

internal/template
  template discovery, metadata, domain rules

internal/render
  profile + templates + optional secret refs -> immutable bundle

internal/bundle
  manifest, checksum helpers, bundle read/write

internal/apply
  diff, backup, atomic write, rollback

internal/server
  HTTP API, routing, auth middleware

internal/web
  server-rendered pages and embedded assets

internal/secret
  local secret references first, external adapters later

internal/domain
  ai and dotfiles domain handlers
```

The canonical layout reference lives in `.trellis/spec/shared/go.md`. Other specs link to it instead of duplicating the tree.

## Language and Runtime

Use Go for the MVP.

Design constraints:

- The product ships as a single `confighub` binary.
- CLI, web UI, API, and server mode share the same core packages.
- The same binary works locally, on a LAN host, and on a VPS.
- No mandatory databases and no mandatory sidecar services in the MVP.
- File-backed storage until requirements justify a database.
- Config parsing validates unknown external input before use.
- Bundle paths are allowlisted and normalized before writes.
- No write operation bypasses diff, backup, checksum verification, and atomic rename.
- External services are reached only through narrow optional adapter interfaces.

## Concurrency Model

Hub side (under single-writer operator assumption):

- The operator is the only writer of `profiles/` and `templates/`.
- The renderer writes bundles into a per-render temp directory under `bundles/.tmp/<bundleVersion>/`, then performs `os.Rename` into `bundles/<profileId>/<bundleVersion>/`. Partially-written bundles are never visible to readers.
- Concurrent readers (web UI, API, client pull) see either the previous bundle or the new one, never a half-written tree.
- `state/` writes (apply log, pull pointer, token store) use the same write-temp + atomic-rename pattern.
- No file locking primitive is required for MVP because there is only one writer, but the renderer must verify "no concurrent renderer is already running" using a `state/render.lock` advisory PID file. If a stale lock is detected, the renderer refuses to overwrite and requires explicit operator action.

Client side:

- `confighub apply` is single-instance per profile; `state/apply.lock` prevents two concurrent applies on the same machine.
- Backups are timestamped, so two simultaneous applies cannot clobber each other even if a lock is bypassed.

## State Directory

```text
state/
  apply.log             # newline-delimited JSON entries; one per applied bundle
  pull/<profile>.json   # last pulled bundle version, etag, checksum, timestamp
  tokens/<id>.json      # token metadata (id, label, scope, hash); never the plaintext token
  render.lock           # hub-side advisory PID lock for active renders
  apply.lock            # client-side advisory PID lock for active applies
```

Rules:

- All files in `state/` are written with mode `0600`.
- `state/` contents must never include raw token values or rendered secret values; only hashes, metadata, and references.
- `apply.log` rotates by size (default 10 MiB) with a single previous file kept.
- `pull/<profile>.json` is per-machine state and is not synced from the hub.

## Data Flow

### Local Render/Apply

```text
profile + templates + optional secret refs
  -> renderer (writes to bundles/.tmp/<version>/)
  -> atomic rename into bundles/<profile>/<version>/
  -> confighub diff --bundle
  -> confighub apply --bundle
  -> backup + atomic write + apply log + rollback pointer
```

### Hosted Hub

```text
reachable machine runs confighub serve
  -> user opens web UI over HTTPS (recommended) or HTTP on loopback
  -> user chooses profile/template domain
  -> user copies bootstrap command or downloads bundle
  -> client pulls metadata, bundle, signature
  -> client verifies signature + manifest + checksums
  -> client runs the local apply flow
```

### Direct Remote Config

```text
tool or wrapper presenting a valid token
  -> GET /api/v1/profiles/{profile}/templates/{template}
  -> server checks template delivery.remote and authorization scope
  -> server streams rendered config bytes with checksum/ETag header
```

Direct remote config is optional per template. Local sync remains the baseline for tools that only read local files.

## Template Domains

### AI Domain

Targets:

- Codex config TOML.
- Claude settings JSON.
- OpenCode JSON.
- Gemini config after exact target files are confirmed.
- Provider endpoint profiles.
- MCP endpoint profiles.

The AI domain should publish endpoint configuration first. Full provider gateway or MCP gateway integration is optional later work.

#### AI Secret Handling

AI configuration files often contain provider API keys, OAuth tokens, or MCP credentials. These rules apply specifically to the AI domain:

- A template that may render secret-derived values must declare `safety.secrets: allowed` and the bundle manifest must mark the file entry accordingly.
- Bundles containing files with `secrets: allowed` may not be downloaded over an unauthenticated transport. Loopback is allowed; non-loopback requires a valid token.
- Direct remote reads of `secrets: allowed` templates require both `delivery: remote` opt-in **and** an authenticated requester. Unauthenticated reads return `404` (never reveal existence by `401`).
- Apply logs and audit logs record only the file's checksum and target path, never any rendered value.
- Bundles at rest on the hub are stored on a filesystem mode `0700` directory owned by the operator account. Encryption at rest is the operator's responsibility (full-disk encryption, encrypted volume); ConfigHub does not encrypt bundle files itself in MVP.
- Operator must use HTTPS (reverse proxy or built-in TLS) for any non-LAN exposure. README and `serve` startup warn if a non-loopback bind starts without TLS.

### Dotfiles Domain

Targets:

- Safe selected `.*` templates or generated fragments.
- Git config fragments.
- Shell fragments.
- Editor/terminal settings when target ownership is explicit.
- Shared helper scripts.

Forbidden:

- Private keys.
- Real local identity files.
- Runtime databases.
- Caches.
- Logs.
- History.
- GUI state.
- `known_hosts`.

#### Fragment Include Strategy

Some dotfiles cannot be safely overwritten (e.g. `~/.gitconfig` is user-owned and accumulates local overrides). For these cases ConfigHub uses a fragment + include pattern:

- The template renders a fragment file at a stable path under `~/.config/confighub/fragments/<domain>/<name>` (e.g. `~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig`).
- The target file (`~/.gitconfig`, `~/.zshrc`, etc.) must include a one-line directive that sources or includes the fragment. This include line is itself a template entry of its own, declared with `safety.includeStrategy: append-once`, which:
  - searches for an existing matching marker line (`# >>> confighub:<name> >>>`);
  - inserts the include block between marker lines if absent;
  - leaves user content above and below the markers untouched;
  - never modifies content outside the marker block on subsequent applies.
- Fragment files are owned by ConfigHub and may be overwritten on every apply.
- Marker blocks are owned by ConfigHub but the host file is owned by the user.

This separation keeps user-owned files mostly untouched while still letting ConfigHub deliver fragments deterministically.

### Settings File Merge Strategy

Tool settings files like `~/.claude/settings.json` or `~/.codex/config.toml` need a per-template merge policy because users add hooks, custom commands, and local paths to them. Each template declares one of:

- `merge: replace` — ConfigHub owns the entire file. User edits are clobbered. The README and template description must call this out.
- `merge: deep-merge` — ConfigHub deep-merges its rendered keys on top of the existing file. Reserved keys (under `confighub.*` namespace) belong to ConfigHub; everything else is preserved.
- `merge: managed-section` — like the fragment include strategy: ConfigHub writes its content inside a marker block (where the file format supports comments) and leaves the rest of the file alone.

MVP supports `replace` and `managed-section`. `deep-merge` is reserved for a later slice once the merge semantics per file format (JSON, TOML, YAML, INI) are spec'd.

## Bundle Contract

Required manifest fields:

- `schemaVersion`
- `bundleVersion`
- `profileId`
- `createdAt`
- `sourceRevision`
- `domains`
- `files`
- `removedFiles`
- `changeSummary`
- `signature` (optional in MVP; required once Slice 5 lands)

Every file entry must include:

- template id
- domain
- relative bundle path
- target path or target path template
- file mode
- checksum (sha256)
- delivery mode (`sync`, `remote`, or both)
- safety policy block (`backup`, `diff`, `symlink`, `secrets`, `merge`, `includeStrategy`)
- whether the file may contain secret-derived values (`secrets: allowed | forbidden`)

### Template Lifecycle (Removed/Renamed)

- Adding a template: appears in `files`.
- Renaming a template: appears in `files` under the new id; old id appears in `removedFiles` with its previous target path.
- Removing a template: appears in `removedFiles` with the previous target path.
- Client apply consults `removedFiles` to delete the corresponding local files (after backup). A removed file that is missing locally is a no-op, not an error.
- Removed files still respect the same backup + apply log + rollback rules as added/updated files.

A concrete manifest JSON example lives in `.trellis/spec/shared/bundle-contract.md`.

## Web/API Surface

MVP web pages:

- home/status page;
- profile list;
- profile detail;
- template/domain list;
- rendered bundle detail (manifest, checksums, removedFiles);
- copyable bootstrap commands;
- warning page for dangerous or disallowed targets.

MVP API:

- `GET /api/v1/status`
- `GET /api/v1/profiles`
- `GET /api/v1/profiles/{id}`
- `GET /api/v1/profiles/{id}/bundle` (returns the latest bundle archive)
- `GET /api/v1/profiles/{id}/bundle/manifest`
- `GET /api/v1/profiles/{id}/bundle/signature`
- `GET /api/v1/profiles/{id}/templates/{templateId}` (direct read; only for `delivery.remote`)

Authentication:

- Loopback (`127.0.0.1`, `::1`) binds may run without a token in development.
- Any non-loopback bind requires `Authorization: Bearer <token>` on every API request.
- Token authenticity is checked via constant-time compare against the stored token hash; plaintext tokens are never stored or logged.
- The web UI uses session cookies derived from token login (HMAC-signed, `Secure`, `HttpOnly`, `SameSite=Lax`); cookies are not required for API access.

API response rules:

- API responses must not leak secret-derived values unless the template explicitly permits and the requester is authorized.
- Direct template reads return raw rendered bytes with the original `Content-Type` recorded on the template entry (default `application/octet-stream`). The response includes `ETag: "<sha256>"` and `X-ConfigHub-Profile: <id>` headers.
- Errors return a JSON body `{ "code": "<machine-code>", "message": "<human-readable>" }` and a meaningful HTTP status.
- Existence of `secrets: allowed` templates is hidden from unauthenticated requesters: `404` is returned, not `401` or `403`, to avoid leaking that the template exists.

## Safety Model

Client apply rules:

- Default to dry-run or diff before writing.
- Non-interactive apply requires `--yes`.
- Every target file is backed up before writing.
- Writes use a temporary file and atomic rename.
- Unknown target paths are rejected.
- Symlink behavior must be explicit per target. Default is reject. Templates that intentionally manage symlinks must declare `safety.symlink: replace` or `safety.symlink: follow`.
- Rollback restores from the most recent successful backup.
- Apply logs must not include raw secret values.
- Domain-specific doctor checks run after apply (`doctor:apply` for write health, `doctor:tools` for installed tool health — see CLI spec).
- Clients verify pulled bundle manifests and checksums even when the hub is trusted.
- From Slice 5 onward, clients also verify bundle signatures before applying.

Symlink-managed dotfiles (e.g. stow, chezmoi) note:

- Default symlink rejection means ConfigHub is not drop-in compatible with stow-style symlink-based dotfiles.
- The recommended interop is to let ConfigHub render fragments under `~/.config/confighub/fragments/...` and let stow/chezmoi manage the host file's include line, OR to give ConfigHub ownership and stop using the symlink manager for those files. The README and dotfiles domain doc must call this out before users adopt the dotfiles domain.

## Optional Adapter Model

The core product must work without Infisical, LiteLLM, Docker MCP Gateway, Docker Compose, Kubernetes, or a database.

Adapters may be added later:

- Secret source adapter: MVP uses local references with optional `age`-encrypted files; external secret stores arrive only if local references are insufficient.
- Provider adapter: publish endpoint profiles first; provider gateway integration only if real routing requirements appear.
- MCP adapter: publish remote MCP endpoint config first; MCP gateway integration only if endpoint publishing is insufficient.
- Reverse proxy adapter: direct server mode first; Caddy/nginx examples ship as `ops/` references when non-LAN deployment begins.

## Rollout / Rollback

Rollout order:

1. Prove local render and local apply against fixture paths.
2. Apply to a temporary test directory.
3. Add hosted read-only web/API with token auth for non-loopback binds.
4. Add pull + bundle signature verification from local/LAN server.
5. Add one AI config template (with `secrets: allowed` policy).
6. Add one safe dotfiles fragment + include strategy.
7. Add VPS deployment docs (TLS via reverse proxy).
8. Add optional adapters only after the single-binary core works.

Rollback:

- `confighub rollback` restores the last backup for a bundle application.
- Manual rollback remains possible because backups are plain files under `~/.confighub/backups/<timestamp>/`.

## Decisions (Previously Open Questions)

- **Repo rename**: rename `ai-config-hub` -> `config-hub` as the last step of `00-bootstrap-guidelines` before any Slice 1 code lands. Go module path: `github.com/ruichen/config-hub` (placeholder until publication).
- **First UI**: server-rendered HTML only, with minimal embedded JavaScript reserved for copy-to-clipboard and diff toggling. No SPA framework in MVP.
- **Local secrets**: MVP uses `age`-encrypted reference files at `~/.config/confighub/secrets/<profile>.age` plus environment-variable references in templates. Keychain integration is a later adapter.
- **First dotfiles**: Git config fragment and Zsh shell fragment via the fragment + include strategy. Editor/terminal templates require a follow-up scope check before Slice 7 ships them.
