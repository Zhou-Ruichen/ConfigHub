# Local Render (Slice 2)

## Goal

Implement `confighub render`: take a profile + the templates it allowlists, render each template into a bundle directory, compute checksums, emit a complete manifest (including `removedFiles` when a previous bundle exists), and atomically promote the bundle into the bundles tree. No serving, no applying, no signing — just deterministic, atomic local render.

## Background

The product direction is locked in the archived `00-bootstrap-guidelines` task. Slice 1 (`05-16-01-go-scaffold`, also archived) shipped the Go module, command stubs, contract types in `internal/profile`/`internal/template`/`internal/bundle`, and fixture data under `examples/`. Slice 2 builds on those types — do not redefine them.

## Scope

### In scope

- `confighub render` command: replaces the Slice 1 stub. Reads a profile + its templates and produces a bundle.
- Template source format: Go `text/template`.
- Template source resolution: the template definition's `source` field is **relative to `<root>/templates/`**, where `<root>` defaults to the current working directory and can be overridden with `--root`.
- Template execution context (Slice 2):
  - `.Profile`: the parsed `profile.Profile` struct.
  - `.Vars`: a `map[string]any` taken from a new optional `vars:` map on the profile.
  - `.Env`: a `map[string]string` of an explicit allowlist of env-var names declared on the template (new optional `envAllowlist: []string` on `template.Template`). Templates may not read arbitrary env vars.
  - `.Secrets`: empty map for Slice 2. Calling `{{ secret "name" }}` in a template returns an error at execute time. Wiring real secrets is Slice 6.
- Atomic write: render into `<root>/bundles/.tmp/<bundleVersion>/`, then `os.Rename` into `<root>/bundles/<profileId>/<bundleVersion>/`. On failure, remove the `.tmp` directory.
- Manifest contract (matches `.trellis/spec/shared/bundle-contract.md`):
  - All Slice 1 fields populated.
  - `signature` is `null` (signing arrives in Slice 5).
  - `removedFiles` populated by comparing against the most-recent existing bundle for the same profile (lexicographically last `bundleVersion`). If no previous bundle exists, `removedFiles` is `[]`.
  - `createdAt` is RFC3339 UTC.
  - `bundleVersion` format: `YYYY-MM-DDTHH-MM-SSZ-NNN`, where `NNN` is a 3-digit sequence within the same second, starting at `001`. If a directory with the candidate version already exists, increment `NNN`.
  - `sourceRevision`: `git:<short-sha>` when the hub root is a git repo; otherwise `none`.
- `checksums.json`: a flat `{"<bundlePath>": "sha256:<hex>"}` map written next to `manifest.json`.
- Advisory lock: acquire `state/render.lock` (under `<root>/state/`) as a PID file before writing anything. If the lock exists and the recorded PID is alive, exit code `13` with a clear message. If the PID is stale, exit `13` and instruct the operator to run `confighub lock release --force` — do not auto-clear.
- `--dry-run` flag: render to memory, print the would-be manifest JSON to stdout, write nothing to disk. Still acquires the render.lock briefly to prevent concurrent operator confusion.
- `--json` flag: on success, emit the manifest JSON to stdout (in addition to the on-disk file).

### Out of scope

- Apply, diff, status, pull, serve — those are later slices.
- Bundle signature creation / verification — Slice 5.
- Target path policy enforcement — Slice 3.
- Secret resolution from `age`-encrypted files / env adapter — Slice 6.
- Conflict detection between hub and client — Slice 4.x (per `.trellis/backlog/conflict-view-and-drag-edit.md`).
- Layered / inherited profiles — deferred (per `.trellis/backlog/local-differentiation.md`).
- Multi-writer locking (RWLock, file lock primitive). Single-operator MVP per `.trellis/spec/shared/concurrency.md`.

## Required Additions to Existing Types

Slice 1 fixed the public shape; Slice 2 adds optional fields (backwards compatible):

- `profile.Profile`: add optional `Vars map[string]any \`yaml:"vars,omitempty"\``.
- `template.Template`: add optional `EnvAllowlist []string \`yaml:"envAllowlist,omitempty"\``.

Update Slice 1 tests if compile breaks (they should not — both fields are optional).

## CLI

```
confighub render \
  --profile <id-or-path> \
  [--root <dir>] \
  [--dry-run] \
  [--json]
```

- `--profile` resolution: if the value contains `/` or ends in `.yaml`/`.yml`, treat as a path. Otherwise resolve to `<root>/profiles/<id>.yaml`.
- `--root` default: current working directory.
- Exit codes per `.trellis/spec/backend/cli.md`:
  - `0` success.
  - `2` usage error.
  - `10` validation error (profile/template/manifest invalid).
  - `13` lock acquisition failure.

## Required Fixture Updates

Slice 1 left template source files unwritten (the template YAMLs reference paths under `templates/...` but the `.tmpl` files don't exist). Slice 2 adds them under `templates/<domain>/<name>/`:

- `templates/ai/claude/settings.json.tmpl` — produces a JSON file with at least `{"managed":"confighub","operator":"{{.Profile.Owner}}"}`. Keep it short; Slice 6 expands the AI domain.
- `templates/dotfiles/git/confighub.gitconfig.tmpl` — produces a fragment with at least `[user]\n\tname = {{.Profile.Owner}}\n`.
- `templates/dotfiles/git/gitconfig-include.tmpl` — produces an include block: a literal `[include]\n\tpath = ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig\n`. (The marker-block wrapping is the apply engine's job in Slice 3; Slice 2 just renders the content.)

Update the existing template YAMLs under `examples/templates/` so their `source:` paths point to these new files (i.e. `source: dotfiles/git/confighub.gitconfig.tmpl` relative to `<root>/templates/`).

Also update `examples/profiles/macbook.yaml` to add an `owner: ruichen` field if Slice 1 didn't already populate it (Slice 1 did — verify), and add a `vars:` map (can be empty or contain one example entry) so the renderer's `.Vars` context can be exercised.

## Acceptance Criteria

- [ ] `confighub render --profile examples/profiles/macbook.yaml --root .` produces a directory under `bundles/macbook/<bundleVersion>/` containing `manifest.json`, `checksums.json`, and the rendered files at the declared bundle paths.
- [ ] The produced manifest matches the bundle contract: all Slice 1 fields populated, `signature: null`, `removedFiles` correctly empty on first render and correctly populated on a subsequent render where a template is removed.
- [ ] Each rendered file's `sha256` checksum matches the corresponding `files[].checksum` entry in the manifest and the `checksums.json` entry.
- [ ] Re-rendering with no changes produces an identical set of files (same checksums) — only `bundleVersion` and `createdAt` change.
- [ ] The `.tmp/<bundleVersion>/` directory is removed on success and on failure; partial output is never visible at the final path.
- [ ] A second concurrent renderer exits with code `13` and an explanatory message; the first renderer is unaffected.
- [ ] `--dry-run` writes nothing under `bundles/` and prints the manifest JSON to stdout.
- [ ] `--json` on a real render also prints the manifest JSON to stdout after writing to disk.
- [ ] Calling `{{ secret "openai_key" }}` in a template fails the render with exit code `10` and a clear "secrets not available in Slice 2" message.
- [ ] `go test ./...` passes; new tests cover: full render round-trip, atomic-rename behavior (induced failure mid-render leaves no partial bundle), lock contention, removed-files detection across two consecutive renders.
- [ ] `go vet ./...` clean.

## References

- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/implement.md` — Slice 2 detailed checklist.
- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/design.md` — concurrency, atomic-rename, lifecycle.
- `.trellis/spec/shared/bundle-contract.md` — manifest fields and worked example.
- `.trellis/spec/shared/concurrency.md` — render.lock, atomic write pattern.
- `.trellis/spec/backend/cli.md` — flag and exit-code conventions.
- `.trellis/spec/backend/service-architecture.md` — package boundaries.

## Notes for the Implementer (Pi)

- Do not commit. Leave changes uncommitted; the orchestrator commits after review.
- Touch only files needed for Slice 2:
  - `cmd/confighub/render.go` (or expand the existing stub file).
  - `internal/render/*.go` — the actual renderer lives here. Other packages should only get small additions if necessary.
  - `internal/bundle/*.go` — extend for `removedFiles` computation, atomic-rename helper, checksum writer; do not break Slice 1 types.
  - `internal/profile/*.go` and `internal/template/*.go` — add the optional `Vars` / `EnvAllowlist` fields only.
  - `templates/...` (new) and `examples/...` (updates to template YAMLs and profile if needed).
  - New `*_test.go` files alongside the new code.
- Do not modify `.trellis/`, `.claude/`, `.codex/`, `.agents/`, `README.md`, `AGENTS.md`, or other slices' fixtures.
- Reuse Slice 1's fixture data and types. Do not rewrite the manifest JSON that Slice 1 produced; that fixture stays for now and Slice 2 may produce additional rendered fixtures alongside it.
- Use `text/template`. Register `secret`, `env`, `var` template functions where `secret` always errors in Slice 2, `env` reads only from the template's `envAllowlist`, and `var` looks up `.Vars`.
- When unsure about a contract detail, prefer the worked example in `bundle-contract.md` literally. If still unclear, leave `// TODO(slice-2): <question>` and continue.

## Reporting

When done, print:

1. List of files created / modified (path only).
2. `go test ./...` output (summary line per package).
3. `go vet ./...` output (or "clean").
4. A sample run output: `confighub render --profile examples/profiles/macbook.yaml --root .` (sanitized).
5. Any TODO comments left, with file:line.
6. Any decision you made that wasn't covered in the spec.
