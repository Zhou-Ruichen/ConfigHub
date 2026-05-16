# Implementation Checklist

## Current Decision

Build **ConfigHub** as a lightweight Go single-binary product named `confighub`, for a **single operator with multiple machines**.

The default deployment is a reachable machine running `confighub serve`. Other machines owned by the same operator consume configuration by pulling rendered bundles or directly reading hub endpoints when a template supports remote delivery.

AI config and dotfiles config are template domains under the same profile/render/bundle/apply model.

This file is the sole source of executable slice checklists. The PRD lists outcomes only.

## Validation Baseline (applies to every slice)

- `go test ./...`
- `go vet ./...`
- Render/diff/apply tests use `t.TempDir()`; no real home-directory writes until Slice 3's acceptance gate passes.

## Slice 0: Bootstrap and Direction

Status: in-progress in `00-bootstrap-guidelines`. Done when:

- [x] README, PRD, design, implement, and specs reflect the ConfigHub direction.
- [x] Naming and Go module path are locked.
- [x] Spec tree contains CLI, concurrency, state, AI secret handling, fragment include strategy, and lifecycle docs.
- [ ] Repo directory renamed `ai-config-hub` -> `config-hub` (final action of the task).

## Slice 1: Go Scaffold and Contracts

Tracked under follow-up task `01-go-scaffold`.

- [ ] `go mod init github.com/ruichen/config-hub`.
- [ ] Create `cmd/confighub`.
- [ ] Add CLI command skeleton for `serve`, `init`, `render`, `pull`, `status`, `diff`, `apply`, `rollback`, `doctor`.
- [ ] Create `internal/profile`, `internal/template`, `internal/render`, `internal/bundle`, `internal/apply`, `internal/server`, `internal/web`, `internal/secret`, `internal/domain`.
- [ ] Add fixture profiles, templates, and bundle examples under `examples/`.
- [ ] Add table-driven tests for profile parsing, template parsing, and bundle manifest validation (including `removedFiles`).

**Done when**: `go test ./...` passes for contract tests; `confighub --help` lists all command stubs; fixture profile/template/bundle round-trip through parse/validate without error.

## Slice 2: Local Render

- [ ] Load a profile from YAML or JSON.
- [ ] Load template definitions from a directory.
- [ ] Render AI and dotfiles templates into `bundles/.tmp/<version>/` then atomic-rename into `bundles/<profile>/<version>/`.
- [ ] Write `manifest.json` (including `removedFiles` and empty `signature` placeholder).
- [ ] Write `checksums.json`.
- [ ] Reject templates that target forbidden paths.
- [ ] Reject templates that include secret-derived values unless `safety.secrets: allowed` is declared.
- [ ] Honor `state/render.lock` advisory PID lock.

**Done when**: `confighub render --profile examples/macbook` produces a bundle with all required manifest fields, all rendered files have matching checksums, and re-running with the same inputs is deterministic (except `createdAt` and `bundleVersion`).

## Slice 3: Local Apply

- [ ] Implement `confighub status`.
- [ ] Implement `confighub diff --bundle <path>`.
- [ ] Implement `confighub apply --bundle <path>` with diff, backup, atomic write, and apply log.
- [ ] Implement timestamped backup at `~/.confighub/backups/<timestamp>/`.
- [ ] Implement rollback pointer in `state/`.
- [ ] Implement `confighub rollback`.
- [ ] Implement `confighub doctor:apply` (write-path health checks).
- [ ] Honor `state/apply.lock` per profile.
- [ ] Honor `removedFiles` (back up then delete).
- [ ] Reject symlink targets unless template declares `safety.symlink: replace | follow`.
- [ ] Apply log redacts secret-derived values.

**Done when**: against a `t.TempDir()` target, a render -> diff -> apply -> modify-target -> apply (no-op) -> rollback flow succeeds; checksum mismatch, forbidden target, symlink rejection, and rollback-after-partial-failure tests all pass.

## Slice 4: Serve Mode and Web UI

- [ ] Implement `confighub serve --bind <addr> --root <dir>`.
- [ ] Add status page, profile list, profile detail, template list, bundle detail (including `removedFiles`), bootstrap command page, and warning page for unsafe templates.
- [ ] Add API routes `GET /api/v1/status`, `/profiles`, `/profiles/{id}`, `/profiles/{id}/bundle`, `/profiles/{id}/bundle/manifest`, `/profiles/{id}/templates/{templateId}`.
- [ ] **Token authentication is required for any non-loopback bind from this slice.** Loopback may run permissive in development.
- [ ] Token store under `state/tokens/<id>.json` keeps hashes only; plaintext tokens are produced once on creation and never persisted.
- [ ] Warn at startup if a non-loopback bind starts without TLS.
- [ ] Pages render usefully without client-side JavaScript; copy-to-clipboard and diff-toggle are the only JS allowed.
- [ ] HTTP-level redaction: `secrets: allowed` templates 404 to unauthenticated requesters.

**Done when**: `confighub serve --bind 127.0.0.1:8787` serves the UI; `confighub serve --bind 0.0.0.0:8787` without a token configured exits non-zero with a clear error; rendered bundle and direct template routes return expected bytes with `ETag` headers; serving over HTTP on a non-loopback bind prints a TLS warning.

## Slice 5: Pull Flow and Bundle Signatures

- [ ] Implement `confighub init --from <url> --profile <id> --token <token>`.
- [ ] Implement `confighub pull --dry-run`.
- [ ] Cache pending bundle metadata and files under `state/pull/<profile>/`.
- [ ] Verify bundle signature after pull (signature is required from this slice onward).
- [ ] Verify manifest and checksums after pull.
- [ ] Reuse the same diff/apply path for pulled bundles (no separate apply path).
- [ ] Reject pulled bundles whose schema version is unsupported.

**Done when**: against a local hub fixture, `init` + `pull` + `diff` + `apply` succeed end-to-end; tampering with manifest, checksum, or signature each cause apply to refuse; clients without a valid token receive `401`/`404` as appropriate.

## Slice 6: AI Domain

- [ ] Add Codex config target (`merge: replace` by default, with operator override).
- [ ] Add Claude settings target (`merge: managed-section` so user customization is preserved).
- [ ] Add OpenCode config target.
- [ ] Add provider endpoint profile rendering.
- [ ] Add MCP endpoint profile rendering.
- [ ] Add direct-read endpoints for templates that opt into `delivery: remote`.
- [ ] Add `confighub doctor:tools` for installed AI CLI health.
- [ ] Apply log redaction tests for every secret-bearing template.

**Done when**: rendering an AI profile with an API key reference produces a manifest entry with `secrets: allowed`; the rendered file contains the resolved key only on disk; apply log + audit log contain no secret value; unauthenticated remote read of a secret-bearing template returns `404`.

## Slice 7: Dotfiles Domain

- [ ] Implement fragment file rendering under `~/.config/confighub/fragments/<domain>/<name>`.
- [ ] Implement `safety.includeStrategy: append-once` for include-line installation.
- [ ] Add Git config fragment template.
- [ ] Add Zsh fragment template.
- [ ] Document forbidden dotfiles and runtime paths in spec.
- [ ] Reject any dotfile target that matches a forbidden-path pattern.
- [ ] Editor/terminal templates require an explicit scope review before they land here.

**Done when**: applying the dotfiles profile creates the fragment file, inserts the marker block in `~/.gitconfig` and `~/.zshrc`, and a second apply is a no-op for the marker block; manually edited content above/below the marker block survives apply and rollback.

## Slice 8: Hardening and Release

- [ ] Add audit log rotation and retention policy.
- [ ] Add restore tests (simulate apply failure, verify rollback).
- [ ] Add backup retention policy (default: keep last 10 per profile; configurable).
- [ ] Add release build commands for macOS and Linux (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`).
- [ ] Add systemd unit example under `ops/`.
- [ ] Add Caddy and nginx reverse-proxy examples under `ops/`.
- [ ] Add optional external secret-store adapter only if it answers a concrete operator need.
- [ ] Add optional provider/MCP gateway adapters only if endpoint templates are insufficient.

**Done when**: a release build runs on each target OS; restore tests pass under simulated mid-apply crash; backup retention purges only by policy; `ops/` examples are minimum-viable TLS configs.

## Review Gates (cross-cutting)

- Do not implement real home-directory writes until Slice 3's acceptance gate passes against a temp directory.
- Do not enable non-loopback `serve` until Slice 4's token auth + TLS warning are in.
- Do not enable pull beyond a fixture hub until Slice 5's signature verification works.
- Do not include dotfiles templates that can contain private/local/runtime state.
- Do not treat hub pull as trusted; clients always verify manifest, checksums, and (from Slice 5) signature.
- Do not build a complex frontend before the server-rendered UI proves the workflow.
