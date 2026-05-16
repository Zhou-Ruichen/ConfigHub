# ConfigHub

ConfigHub is a lightweight self-hosted configuration hub for a **single operator with multiple machines**. Deploy it on one machine you control, then publish, inspect, download, sync, and safely apply configuration templates from any of your other machines.

The first template domains are:

- `ai`: Codex, Claude, OpenCode, Gemini, provider endpoints, MCP endpoints, and AI helper files.
- `dotfiles`: selected `.*` files and fragments such as shell, Git, editor, terminal, and other shared environment templates.

The product goal is "open and use": you open the hub URL, see available profiles/templates, copy the bootstrap command, and sync a device without understanding the internal repository layout.

## Target User

ConfigHub MVP is built for one person who owns the hub and every client machine that consumes from it. Multi-user accounts, RBAC, and organization-level isolation are out of MVP scope. If you need those, ConfigHub is not the right tool yet.

## Core Idea

ConfigHub separates three concerns:

1. **Template source**: versioned templates and profiles maintained on the hub.
2. **Rendered bundle**: immutable per-device/per-user output with manifest, checksums, and (from Slice 5) signatures.
3. **Consumption mode**: devices either sync bundles into local files or directly read remote config endpoints when a template opts in.

```text
Reachable machine
  confighub serve
    profiles/
    templates/
      ai/
      dotfiles/
    bundles/
    state/
    web UI + HTTP API

Client machines
  confighub init --from <hub-url> --token <token>
  confighub pull --dry-run
  confighub diff
  confighub apply
  confighub rollback

Tools that support remote config
  read hub endpoints directly (per-template opt-in, token-scoped)
```

## Naming

- Product: **ConfigHub**.
- Repository slug: **`config-hub`**.
- Command: **`confighub`**.
- Go module: `github.com/ruichen/config-hub` (placeholder until publication).

## Install

Until releases are published, build from source:

```bash
git clone https://github.com/ruichen/config-hub.git
cd config-hub
go install ./cmd/confighub
```

Verify:

```bash
confighub --help
```

Pre-built binaries for `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64` ship from Slice 8 onward.

## Security Defaults (read before exposing the hub)

ConfigHub is safe by default on loopback. Before binding to anything other than `127.0.0.1`:

- A token must be configured. `confighub serve --bind 0.0.0.0:<port>` without a token exits non-zero.
- TLS is strongly recommended. Use a reverse proxy (Caddy or nginx examples land in Slice 8) or built-in TLS once available. The server prints a warning on startup if a non-loopback bind starts over plain HTTP.
- Tokens are scoped (`pull:<profile>`, `read:<template>`, `admin`). Give each device the narrowest scope it needs.
- Templates that may render secret-derived values (Claude/Codex/OpenCode/MCP credentials) are tagged `safety.secrets: allowed`. Unauthenticated requests for those templates return `404` — existence is hidden by design.

If you intend to expose the hub on a LAN, VPN, or VPS, finish reading [Security](#security-defaults-read-before-exposing-the-hub) before running `serve`.

## User Workflows

### Deploy Once

1. Install the binary on the hub machine.
2. Populate the hub root (`profiles/`, `templates/`) from your repository: either `git clone` your config repo and `cd` into it, or copy the directories into a working `--root` directory. ConfigHub does not yet manage uploads, so the templates live on the hub's filesystem.
3. Create at least one token: `confighub token create --label macbook --scope pull:macbook` (commands stabilize in Slice 4; output includes the plaintext token exactly once).
4. Start the hub:

```bash
confighub serve --bind 127.0.0.1:8787 --root /var/lib/confighub
```

Open:

```text
http://localhost:8787
```

The UI shows:

- available profiles;
- available template domains;
- rendered bundle versions, manifest, `removedFiles`, and checksums;
- device bootstrap commands;
- direct-read URLs where enabled;
- warnings for templates that write sensitive paths.

For LAN or VPS exposure, replace `127.0.0.1` with the appropriate bind and put a TLS-terminating reverse proxy in front. The server refuses non-loopback binds without a token configured.

### Sync A Device

```bash
confighub init --from http://config.example.local:8787 --profile macbook --token <token>
confighub pull --dry-run
confighub diff
confighub apply
```

The token is stored on the client machine under `~/.config/confighub/state/tokens/<id>.json` (mode `0600`) — only its hash; the plaintext you pasted is discarded after registration.

### Direct Remote Config

Some tools may read configuration directly from the hub instead of writing local files:

```text
http://config.example.local:8787/api/v1/profiles/macbook/templates/ai/codex/config.toml
http://config.example.local:8787/api/v1/profiles/macbook/templates/dotfiles/git/config
```

Direct remote config is optional per template (`delivery: remote`). Local sync remains the compatibility baseline because most CLI tools only read local files. Direct reads of secret-bearing templates require a scoped token; unauthenticated requests return `404`.

## Scope

### In Scope

- Host template profiles on a reachable machine.
- Render per-user, per-device, or per-role bundles.
- Cover AI configuration and selected `.*` file templates.
- Provide a small server-rendered web UI for browsing, copying bootstrap commands, and inspecting rendered output.
- Provide an HTTP API for bundle metadata, downloads, and direct template access.
- Provide a CLI for `init`, `pull`, `diff`, `apply`, `rollback`, `doctor:apply`, and `doctor:tools`.
- Verify manifests, checksums, and (from Slice 5) bundle signatures before apply.
- Back up local files before writes; atomic rename on commit.
- Support local, LAN, and VPS deployment with the same binary.
- Support template lifecycle (add / rename / remove) via manifest `files` and `removedFiles`.

### Out of Scope

- Multi-user accounts, RBAC, or organization-level isolation.
- Blind full-home-directory sync.
- Syncing sessions, cookies, history, caches, local databases, private keys, `known_hosts`, or GUI state.
- Requiring Kubernetes, Docker Compose, LiteLLM, Infisical, Docker MCP Gateway, or any database for the MVP.
- Replacing Dotfiles as a source repository for shared environment files.
- Building a full LLM proxy or MCP gateway before direct endpoint templates are insufficient.
- Windows support (not refused; just not validated in MVP).

## Architecture

```text
┌────────────────────────────────────────────┐
│ Reachable host: VPS, office machine, LAN    │
│                                            │
│  confighub serve                           │
│   ├─ web UI                                │
│   ├─ HTTP API (token auth for non-loopback)│
│   ├─ profiles/                             │
│   ├─ templates/                            │
│   │   ├─ ai/                               │
│   │   └─ dotfiles/                         │
│   ├─ renderer (writes via tmp+rename)      │
│   ├─ bundle store                          │
│   ├─ state/ (apply log, tokens, locks)     │
│   └─ optional secret adapter               │
└────────────────┬───────────────────────────┘
                 │ pull / direct read
                 ▼
┌────────────────────────────────────────────┐
│ Client device                              │
│                                            │
│  confighub init                            │
│  confighub pull --dry-run                  │
│  confighub diff                            │
│  confighub apply                           │
│  confighub rollback                        │
│  local backup + apply log                  │
└────────────────────────────────────────────┘
```

## Technology Choice

MVP language: **Go**.

Why Go fits:

- One static binary can provide server, web UI, API, and client CLI.
- The same artifact runs on a VPS, LAN host, workstation, and client machines.
- Fast startup and low memory use are a good fit for "open and use" deployment.
- The standard library covers HTTP, embedded assets, templates, files, checksums, process execution, and cross-platform path handling.
- Client machines do not need Node, Deno, Python virtualenvs, or a runtime-specific package manager.
- The hardest parts are safe filesystem writes, verification, rollback, and simple distribution, not a complex frontend framework.

Stack:

- Go CLI/server binary.
- Server-rendered HTML with embedded static assets; minimal embedded JavaScript for copy-to-clipboard and diff toggle only.
- File-backed storage for profiles, templates, bundles, and state metadata under a single-writer (operator) assumption.
- Optional adapters for external secret stores, provider gateways, MCP gateways, or reverse proxies later.

Layout:

```text
cmd/confighub/          # CLI entrypoint and serve mode
internal/profile/       # profile parsing and validation
internal/template/      # template definitions, domain rules
internal/render/        # profile + templates -> rendered bundle
internal/bundle/        # manifest, checksums, removedFiles, read/write
internal/apply/         # diff, backup, atomic write, rollback, marker blocks
internal/server/        # HTTP API and server routing
internal/web/           # server-rendered UI and embedded assets
internal/secret/        # local secret references first, adapters later
internal/domain/        # ai, dotfiles, future domains
examples/               # sample profiles/templates/bundles
ops/                    # systemd / reverse-proxy / TLS examples (Slice 8)
```

## Core Contracts

### Profile

```yaml
id: macbook
owner: ruichen
role: workstation
os: macos
domains:
  ai: true
  dotfiles: true
allowedTemplates:
  - ai/codex
  - ai/claude
  - dotfiles/git
  - dotfiles/zsh
```

### Template

```yaml
id: dotfiles/git-include
domain: dotfiles
source: templates/dotfiles/gitconfig-include.tmpl
target: ~/.gitconfig
mode: "0644"
delivery:
  sync: true
  remote: false
safety:
  diff: required
  backup: required
  symlink: reject
  secrets: forbidden
  merge: managed-section
  includeStrategy: append-once
```

### Bundle

```text
bundles/<profile>/<bundleVersion>/
  manifest.json
  checksums.json
  files/
    ai/codex/config.toml
    ai/claude/settings.json
    dotfiles/git/confighub.gitconfig
    dotfiles/zsh/confighub.zshrc
```

Manifest fields include `schemaVersion`, `bundleVersion`, `profileId`, `createdAt`, `sourceRevision`, `domains`, `files`, `removedFiles`, `changeSummary`, and (from Slice 5) `signature`. A worked example lives in `.trellis/spec/shared/bundle-contract.md`.

## Apply Rules

Every local apply must:

1. Resolve the active profile.
2. Verify manifest and checksums (and signature from Slice 5).
3. Reject files outside allowlisted target policies.
4. Show a diff unless `--yes` is explicitly provided.
5. Back up every target file under `~/.confighub/backups/<timestamp>/` (mode `0700`/`0600`).
6. Write through a temporary file and atomic rename.
7. Process `removedFiles` (back up then delete; refuse if local checksum does not match `previousChecksum`).
8. Run domain-specific doctor checks (`doctor:apply`).
9. Record an apply log with bundle version and rollback pointer.

Remote access is not trusted automatically. Clients verify manifests and checksums on every pull, regardless of who runs the hub.

## Template Domains

### AI

Initial targets:

- `~/.codex/config.toml` (typically `merge: replace`)
- `~/.claude/settings.json` (recommended `merge: managed-section` so user hooks/customizations survive)
- `~/.config/opencode/opencode.json`
- `~/.config/opencode/oh-my-opencode-slim.json`
- provider endpoint profiles
- MCP endpoint profiles

AI configs often carry API keys or OAuth credentials. Those templates declare `safety.secrets: allowed` and follow the secret-handling guide in `.trellis/spec/big-question/secret-handling.md`. They are not delivered over unauthenticated transport.

### Dotfiles

Initial targets are conservative:

- generated Git config fragments (via the fragment + include strategy);
- generated shell fragments (Zsh first);
- selected editor or terminal settings only when ownership is clear;
- helper scripts that are already safe to distribute.

Never manage private keys, real local identity files, runtime databases, caches, logs, history, GUI state, or `known_hosts`.

#### Fragment + Include Strategy

For dotfiles that users edit by hand (`~/.gitconfig`, `~/.zshrc`), ConfigHub writes a fragment under `~/.config/confighub/fragments/<domain>/<name>` and inserts a small include block into the host file between marker comments (`# >>> confighub:<name> >>>` ... `# <<< confighub:<name> <<<`). Content above and below the marker block is preserved across applies and rollbacks. The fragment itself is fully owned by ConfigHub and can be overwritten.

#### Symlink-managed Dotfiles (stow, chezmoi)

ConfigHub rejects symlinked target files by default. If you already use stow or chezmoi:

1. Let ConfigHub render fragments under `~/.config/confighub/fragments/...` and let stow/chezmoi manage the include line in the host file.
2. Or hand ownership of the host file to ConfigHub and stop tracking it in stow/chezmoi.

The two tools do not compose drop-in. Pick a strategy per file.

## Milestones

Detailed slices and acceptance gates live in `.trellis/tasks/00-bootstrap-guidelines/implement.md`.

1. **Product direction (Slice 0, this task)**: README, name, domains, deployment model, safety rules, repo rename.
2. **Go scaffold (Slice 1)**: `cmd/confighub` and internal package layout.
3. **Local render (Slice 2)**: fixture profiles/templates -> bundles with manifest, checksums, `removedFiles`.
4. **Local apply (Slice 3)**: diff, backup, atomic write, rollback, `doctor:apply`.
5. **Serve mode (Slice 4)**: web UI and HTTP API, with token auth required for non-loopback binds.
6. **Pull flow (Slice 5)**: init, pull, signature verification, apply.
7. **AI domain (Slice 6)**: Codex, Claude, OpenCode, provider endpoints, MCP endpoints, `doctor:tools`.
8. **Dotfiles domain (Slice 7)**: safe `.*` templates and fragment include strategy.
9. **Hardening (Slice 8)**: audit log retention, restore tests, release builds, optional adapters.

## Reference Projects

These are useful references, not required dependencies:

- chezmoi and ansible-pull for client-side apply patterns.
- Infisical/OpenBao for secret-source adapter design.
- LiteLLM/New API for provider endpoint patterns.
- Docker MCP Gateway and MCP Streamable HTTP for remote MCP configuration patterns.
