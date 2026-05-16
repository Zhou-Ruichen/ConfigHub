# Go Scaffold and Contracts (Slice 1)

## Goal

Lay down the Go module, command skeleton, internal package boundaries, contract types, fixture data, and contract tests for ConfigHub. No business logic yet — just the scaffolding so Slice 2 (render) and Slice 3 (apply) have something to build on.

## Background

The product direction is locked in `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/`. The detailed Slice 1 checklist lives in that task's `implement.md` (Slice 1 section) and is the authoritative todo list for this task. All structural decisions (package boundaries, manifest schema, CLI conventions) are documented under `.trellis/spec/`.

## Scope

In scope:

- `go mod init github.com/ruichen/config-hub`.
- `cmd/confighub` with subcommand skeleton: `serve`, `init`, `render`, `pull`, `status`, `diff`, `apply`, `rollback`, `doctor` (each prints "not implemented" but registers help text per `.trellis/spec/backend/cli.md`).
- `internal/profile`, `internal/template`, `internal/render`, `internal/bundle`, `internal/apply`, `internal/server`, `internal/web`, `internal/secret`, `internal/domain` with package docs and minimal stubs.
- Profile, Template, Manifest, FileEntry, RemovedFileEntry, Safety, Delivery struct types matching `.trellis/spec/shared/bundle-contract.md` exactly (JSON/YAML tags consistent with the worked example there).
- Fixture data under `examples/`: at least one profile (`examples/profiles/macbook.yaml`), two templates (one AI with `secrets: allowed`, one dotfiles fragment), and one rendered bundle directory with `manifest.json` matching the worked example in `bundle-contract.md`.
- Table-driven tests in `internal/profile`, `internal/template`, `internal/bundle` covering: valid parse round-trip, invalid input rejection, manifest with/without `removedFiles`, manifest with empty/null `signature`, schema-version mismatch rejection.
- `--help`, `--version`, and the documented exit codes (per `cli.md` §"Exit Codes") wired through.

Out of scope:

- Actual rendering, applying, serving, or pulling — those are Slices 2-5.
- Token management commands (`confighub token ...`) — Slice 4.
- Bundle signature verification — Slice 5.
- AI / dotfiles template content beyond fixtures — Slices 6-7.

## CLI Library Decision

Use `github.com/spf13/cobra` for command registration and `pflag` (cobra's default) for flags. Standard, widely audited, and matches the multi-subcommand structure already documented in `cli.md`.

## Acceptance Criteria

- [ ] `go mod tidy` succeeds with `go 1.22+` (or whichever recent toolchain is installed; record it).
- [ ] `confighub --help` lists every documented subcommand.
- [ ] `confighub --version` prints a version string (placeholder `0.1.0-dev` is fine).
- [ ] Each documented subcommand returns a non-implemented stub with exit code `0` for `--help` and an explicit "not implemented yet" message + exit code `1` for real invocation.
- [ ] Loading `examples/profiles/macbook.yaml` returns a non-zero-value `Profile` struct via the `profile` package's `Load` function.
- [ ] Loading the fixture bundle manifest returns a valid `Manifest` struct via the `bundle` package's `LoadManifest` function.
- [ ] Manifest with unsupported `schemaVersion` is rejected at parse time.
- [ ] Manifest entry whose `safety.secrets` is unset defaults to `forbidden`.
- [ ] `go test ./...` passes with at least one table-driven test per package listed above.
- [ ] `go vet ./...` passes with no warnings.

## References

- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/implement.md` — Slice 1 detailed checklist.
- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/design.md` — package boundaries and concurrency rules.
- `.trellis/spec/shared/bundle-contract.md` — manifest schema and worked example.
- `.trellis/spec/shared/go.md` — Go layout, errors, tests.
- `.trellis/spec/backend/cli.md` — subcommand, flag, exit-code conventions.
- `.trellis/spec/backend/service-architecture.md` — package boundary rules.

## Notes for the Implementer (Pi)

- Do not commit. Leave changes staged or unstaged in the working tree; the orchestrator (Claude) commits after review.
- Touch only the files needed for Slice 1. Do not edit specs, README, or other task artifacts.
- When unsure about a contract detail, prefer the worked example in `bundle-contract.md` over reinterpreting prose. If still unclear, leave a TODO comment with `// TODO(slice-1):` and continue.
- Keep fixture files small but realistic — they will be reused in Slices 2 and 3.
