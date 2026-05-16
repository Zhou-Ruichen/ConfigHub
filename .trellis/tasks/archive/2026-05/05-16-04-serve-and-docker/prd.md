# Serve Mode + Web UI + Docker (Slice 4)

## Goal

Stand up `confighub serve`: an HTTP server that exposes a minimal admin Web UI (server-rendered HTML) and an HTTP API for reading profiles, bundle metadata, bundle archives, and individual rendered templates. Add token authentication (required for non-loopback binds), token CRUD commands, a multi-stage Dockerfile, a systemd unit, a Caddy reverse-proxy example, and a short deploy guide so the hub can be shipped to the operator's VPS.

After Slice 4, the operator can run `confighub serve` locally **or** `docker run confighub` on `hk-cn2` and reach a working hub at an HTTP endpoint. Slice 5 (pull + signature) will then make the hub useful from other machines.

## Background

- Slices 0–3 are archived under `.trellis/tasks/archive/2026-05/`. The Slice 3 apply engine, the Slice 2 renderer, and the Slice 1 contract types are unchanged here — Slice 4 is purely additive on top.
- The deployment target is the operator's VPS `hk-cn2` (has Docker, no Go). Slice 4 must produce artifacts that build locally and ship cleanly to that VPS.

## Scope

### In scope

- `confighub serve --bind <addr> --root <dir>` command, replacing the Slice 1 stub.
- HTTP server with the routes listed below.
- Server-rendered HTML pages (status, profile list, profile detail, bundle detail, bootstrap command, warning) using `html/template` with embedded assets via `embed.FS`. Minimal embedded JavaScript is allowed only for clipboard copy buttons.
- Token authentication for all non-loopback API access; loopback (`127.0.0.1`, `::1`) may run permissive.
- `confighub token create --label <l> --scope <s>`, `confighub token list`, `confighub token revoke <id>` commands.
- Direct template read endpoint with secret-bearing redaction rules.
- Bundle archive download (single tar.gz over the bundle directory).
- TLS startup warning on non-loopback HTTP binds.
- Multi-stage `Dockerfile` (Go build stage + distroless final stage).
- `ops/systemd/confighub.service` example unit.
- `ops/caddy/Caddyfile.example` for TLS termination via Caddy.
- `ops/README.md` deployment quickstart (local, docker, hk-cn2 docker, systemd).

### Out of scope

- Bundle signature creation / verification — Slice 5.
- Pull flow (`confighub init`, `confighub pull`) — Slice 5.
- Conflict view, drag-edit UI, layered profiles — `.trellis/backlog/` items.
- SPA framework, client-side routing, large JS — explicitly rejected per `spec/frontend/index.md`.
- Multi-user accounts, RBAC fine-grained per-route — only token + simple scope (per `prd` for `00-bootstrap`).
- HTTPS termination inside the binary — Slice 4 prints a warning and recommends reverse proxy; built-in TLS is a later slice if needed.
- Hot reload of profiles/templates — operator restarts the server after edits in MVP.

## CLI

### `confighub serve`

```
confighub serve \
  --bind <addr>       (default 127.0.0.1:8787) \
  --root <dir>        (default .) \
  [--allow-no-token]  (loopback only; rejected on non-loopback binds)
```

- On startup, the server scans `<root>/state/tokens/*.json` for tokens.
- If `--bind` is non-loopback (anything other than `127.0.0.1`, `::1`, `localhost`) and there are zero tokens configured, exit code `21` with a clear error: "non-loopback bind requires at least one token; create one with `confighub token create --label <l> --scope <s>`".
- On non-loopback HTTP bind, log a warning to stderr: "non-loopback bind over plain HTTP; terminate TLS at a reverse proxy (see ops/caddy/Caddyfile.example) before serving real traffic".
- Sigterm / sigint trigger graceful shutdown (close listener, wait for in-flight handlers up to 10 s).

### Token commands

```
confighub token create --label <label> --scope <scope> [--root <dir>]
confighub token list   [--root <dir>] [--json]
confighub token revoke <id> [--root <dir>]
```

- `create` prints the plaintext token **once** to stdout (one line, no extra formatting) and writes the hash + metadata to `<root>/state/tokens/<id>.json`. Token id is a short random suffix appended to a `cfh_` prefix; the plaintext is `cfh_` + 32 random bytes base64url-encoded.
- `list` prints id, label, scope, createdAt (never the plaintext or hash). `--json` emits structured output.
- `revoke <id>` deletes `state/tokens/<id>.json` (silent success) or exits non-zero if missing.
- Scope syntax for Slice 4: `pull:<profileId>`, `read:<templateId>`, `admin`. `admin` grants everything. Other scopes are rejected with exit 10.

### Updated exit codes

Per `.trellis/spec/backend/cli.md`. New ones used by serve:

- `21` — non-loopback bind without any token configured.
- `22` — TLS cert/key load failure (reserved; not used in Slice 4 since there is no built-in TLS yet).

## HTTP API

| Method | Path | Body | Auth | Notes |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/status` | JSON | none | `{ "version": "...", "profiles": N, "tokens": N, "uptime": "1h2m" }` |
| `GET` | `/api/v1/profiles` | JSON | required on non-loopback (any token) | List of `{id, owner, role, domains}` |
| `GET` | `/api/v1/profiles/{id}` | JSON | `pull:<id>` or `admin` (non-loopback) | Profile detail + latest bundle version |
| `GET` | `/api/v1/profiles/{id}/bundle` | `application/gzip` (tar.gz) | `pull:<id>` or `admin` | Stream the latest bundle dir as tar.gz |
| `GET` | `/api/v1/profiles/{id}/bundle/manifest` | JSON | `pull:<id>` or `admin` | Latest bundle manifest |
| `GET` | `/api/v1/profiles/{id}/bundle/signature` | — | — | Always `404` in Slice 4 (signing arrives in Slice 5) |
| `GET` | `/api/v1/profiles/{id}/templates/{templateId}` | template content-type | `read:<templateId>` or `admin` if `delivery.remote`; secret-bearing requires the matching token; otherwise `404` | Direct rendered template bytes |

Response rules:

- Errors: JSON `{ "code": "<machine-code>", "message": "..." }` plus a meaningful HTTP status.
- `Content-Type` for direct template reads defaults to `application/octet-stream` if the template entry does not declare one. (Slice 4 keeps it simple — we always serve `application/octet-stream` for now; refining per-template Content-Type is Slice 6.)
- Every binary response (bundle archive, direct template) carries `ETag: "<sha256-hex>"`. `If-None-Match` matching returns `304 Not Modified`.
- Bundle archive content is computed on the fly from the latest bundle dir (no pre-built artifact on disk).
- Secret-bearing templates (`safety.secrets: allowed`) return `404` to unauthenticated requesters — never `401` or `403`. Authorized requesters get `200`.

Authentication:

- Loopback bind (`127.0.0.1`, `::1`): API may run without a token; all endpoints accessible. The web UI is the same.
- Non-loopback bind: every request must carry `Authorization: Bearer <plaintext>`; the server hashes (`sha256`) and looks up by hash; missing/wrong → `401` with `{code:"unauthorized"}` (except secret-bearing template reads, which 404 as above).

## Web UI

Pages (all server-rendered HTML, no SPA):

| Path | Title | Content |
| --- | --- | --- |
| `/` | Status | Hub status, profile count, bundle count, token count, links. |
| `/profiles` | Profiles | Table of profiles with bundle counts and last-render timestamp. |
| `/profiles/{id}` | Profile detail | Profile YAML preview, allowed templates, latest bundle link, bootstrap command. |
| `/profiles/{id}/bundles/{version}` | Bundle | Manifest (file list with checksums, removedFiles, signature placeholder), download link, copyable bootstrap command. |
| `/profiles/{id}/bootstrap` | Bootstrap command | Single copy-button page with one-line client init command. |
| `/warnings` | Warnings | List of currently dangerous things (no tokens on a non-loopback bind, templates with `secrets: allowed`, missing bundles for declared profiles). |

Rules:

- All HTML auto-escaped via `html/template`.
- Layout: one base template (`layout.html.tmpl`) with navigation; each page extends it.
- Visual direction per `spec/frontend/admin-ui.md`: compact tables, monospace for commands/paths/checksums, restrained color, no marketing hero.
- One small inline `<script>` for clipboard copy buttons (vanilla, ≤ 50 lines, no external deps).
- No CSS framework; ship a small (< 5 KB) embedded stylesheet.
- Web routes are gated by the same auth rules as their API counterparts on non-loopback binds (token in cookie or `Authorization` header). For Slice 4, simplest path: web routes accept `?token=<plaintext>` on first hit, which sets an HMAC-signed `confighub_session` cookie. Cookie is `Secure` (when behind TLS) / `HttpOnly` / `SameSite=Lax`. This is the minimum to make the web UI usable behind a non-loopback bind without forcing curl-style usage.

## Token Store

Files at `<root>/state/tokens/<id>.json`, mode `0600`:

```json
{
  "id": "cfh_abc123",
  "label": "macbook",
  "scope": "pull:macbook",
  "hash": "sha256:<hex>",
  "createdAt": "2026-05-16T18:00:00Z"
}
```

Hash is `sha256(plaintext)` hex-encoded. Plaintext is never persisted.

Plaintext format: `cfh_` + base64url(32 random bytes), no padding. Length ~47 chars.

## Docker

`Dockerfile` (multi-stage):

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/confighub ./cmd/confighub

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/confighub /usr/local/bin/confighub
WORKDIR /var/lib/confighub
USER nonroot
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/confighub"]
CMD ["serve", "--bind", "0.0.0.0:8787", "--root", "/var/lib/confighub"]
```

Build target on Apple Silicon: `docker buildx build --platform linux/amd64,linux/arm64 -t confighub:dev .` (the operator's `hk-cn2` is likely `linux/amd64`; we still ship both for portability).

`.dockerignore` should exclude `.trellis/`, `.git/`, `bundles/`, `state/`, test outputs.

## Ops Files

- `ops/systemd/confighub.service` — example systemd unit running the binary directly on a host (no Docker). Notes user, working dir, restart policy.
- `ops/caddy/Caddyfile.example` — minimal Caddy config: terminate TLS for `config.example.com`, reverse-proxy to `127.0.0.1:8787`, forward `Authorization` header.
- `ops/README.md` — short guide covering:
  1. Build locally: `docker build -t confighub:dev .`
  2. Ship to hk-cn2: `docker save confighub:dev | gzip | ssh hk-cn2 'gunzip | docker load'`
  3. Run on hk-cn2: example `docker run` with `-v` mounts for `/var/lib/confighub/{profiles,templates,bundles,state}` and a token created first.
  4. Reverse-proxy via Caddy: link to the Caddyfile example.
  5. Or systemd-only: link to the unit, install steps, where to put the binary.

## Required Additions / Updates to Existing Code

- `internal/server/` — HTTP routing, middleware (auth, logging, recover), handlers per route.
- `internal/web/` — embedded HTML templates and assets, page rendering helpers.
- `internal/secret/` (or `internal/token/` — pick one and stay consistent) — token model, hashing, creation, listing, lookup-by-hash, revocation. The Slice 1 stub package `internal/secret` is the natural home; rename to `internal/token` only if it materially clarifies the boundary. Document the choice.
- `cmd/confighub/serve.go`, `cmd/confighub/token.go` — new CLI files. Update `cmd/confighub/main.go` to register the token command tree.
- New tests under `internal/server/` using `httptest.NewServer` against an in-memory hub.

## Acceptance Criteria

- [ ] `confighub serve --bind 127.0.0.1:8787 --root <tmp>` starts; `GET /api/v1/status` returns `200` with a JSON body listing version and profile count.
- [ ] `confighub serve --bind 0.0.0.0:8787 --root <tmp>` without any tokens configured exits `21` with the clear "create a token" message before opening the listener.
- [ ] `confighub serve --bind 0.0.0.0:8787 --root <tmp>` with a token configured starts, and logs a TLS warning to stderr that mentions the Caddy example.
- [ ] On a non-loopback bind, a request without `Authorization` returns `401` for normal endpoints and `404` for secret-bearing template reads.
- [ ] A valid token with `pull:macbook` scope can `GET /api/v1/profiles/macbook/bundle` and receives a `gzip`-encoded tar archive that unpacks to a valid bundle directory; `ETag` and `If-None-Match → 304` both work.
- [ ] A valid token without `read:ai/claude` scope cannot `GET /api/v1/profiles/macbook/templates/ai/claude` (secret-bearing) — returns `404`, not `401` or `403`.
- [ ] `confighub token create --label macbook --scope pull:macbook --root <tmp>` prints exactly one line (the plaintext token) and writes `state/tokens/<id>.json` (mode `0600`); the plaintext does not appear in any file or log.
- [ ] `confighub token list --root <tmp> --json` produces a stable JSON array; `confighub token revoke <id>` removes the file.
- [ ] All web pages render without client-side JavaScript (except the small embedded copy-button script); `curl http://localhost:8787/profiles` returns usable HTML.
- [ ] `docker build -t confighub:dev .` succeeds; the resulting image is under 30 MB; `docker run --rm confighub:dev --version` prints `confighub version 0.1.0-dev`.
- [ ] `docker run --rm -v <tmp>:/var/lib/confighub -p 8787:8787 confighub:dev serve --bind 0.0.0.0:8787 --root /var/lib/confighub` fails with exit 21 when no tokens exist (so the safety check works inside the container).
- [ ] `ops/systemd/confighub.service`, `ops/caddy/Caddyfile.example`, and `ops/README.md` exist and reference real flag names and paths.
- [ ] `go vet ./...` clean; `go test ./...` passes including new `internal/server` tests.
- [ ] All Slice 1–3 tests continue to pass.

## References

- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/implement.md` — Slice 4 detailed checklist.
- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/design.md` — web/API surface, auth, secret handling.
- `.trellis/spec/backend/http-api.md` — route table, response rules, auth.
- `.trellis/spec/backend/security.md` — token model, redaction, TLS.
- `.trellis/spec/backend/state-directory.md` — token store layout.
- `.trellis/spec/frontend/admin-ui.md` — page list, visual direction.
- `.trellis/spec/frontend/quality.md` — UI quality bars.
- `.trellis/spec/big-question/secret-handling.md` — secret-bearing template behavior.
- `.trellis/spec/backend/cli.md` — flag and exit-code conventions.

## Notes for the Implementer (Pi)

- Do not commit. Leave changes uncommitted; the orchestrator commits after review.
- Touch only files needed for Slice 4:
  - `cmd/confighub/serve.go`, `cmd/confighub/token.go`, `cmd/confighub/main.go` (register new commands).
  - `internal/server/*.go` — handlers, routing, middleware, tests.
  - `internal/web/*.go` and `internal/web/assets/*` — templates, embed, page helpers.
  - `internal/secret/*.go` — token store, hashing, CRUD. (Stay in `internal/secret`; do not rename packages.)
  - `Dockerfile`, `.dockerignore`.
  - `ops/systemd/confighub.service`, `ops/caddy/Caddyfile.example`, `ops/README.md`.
- Do not modify `.trellis/`, `.claude/`, `.codex/`, `.agents/`, `README.md`, `AGENTS.md`, fixture YAMLs, or other slices' work. Slice 1–3 tests must keep passing without modification.
- Stick to the standard library where possible. `cobra` and `yaml.v3` are already in go.mod; do not add web frameworks. Use stdlib `net/http`, `html/template`, `embed`, `archive/tar`, `compress/gzip`, `crypto/sha256`, `crypto/rand`, `encoding/base64`, `net/http/httptest`.
- Token plaintext logging is forbidden anywhere. Tests should include negative assertions that no log line and no `state/tokens/*.json` contains the plaintext.
- Web UI behaves under `httptest` even without JS — write tests using `http.Get` and check the body for substring matches.
- Pages should render with empty fixture state (no profiles, no bundles) without panicking.
- When ambiguous, follow the spec links above literally. If still unclear, leave `// TODO(slice-4): <question>` and continue.

## Reporting

When done, print:

1. List of files created / modified (path only).
2. `go test ./...` output (summary line per package).
3. `go vet ./...` output (or "clean").
4. Local smoke output: start `confighub serve --bind 127.0.0.1:8787 --root <tmp>`, hit `/api/v1/status` and `/profiles` and one bundle URL, show the response codes/bodies.
5. `docker build` output's final image size line.
6. Any TODO comments you left, with `file:line`.
7. Any decision you made that wasn't covered in spec or PRD.
