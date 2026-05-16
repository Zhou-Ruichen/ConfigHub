# Local Apply (Slice 3)

## Goal

Implement the client-side write path: take a bundle produced by Slice 2's renderer, verify it, diff it against the local filesystem, back up the existing targets, atomically write the new content, record what happened, and provide rollback. Plus the read-only inspection commands `status` and `diff`, the post-apply health check `doctor:apply`, and the recovery command `rollback`.

After Slice 3, an operator can run `confighub render` (Slice 2) + `confighub apply` (Slice 3) end-to-end on one machine and see their `~/.gitconfig` / `~/.claude/settings.json` updated safely.

## Background

The product direction and the bundle contract are locked in the archived `00-bootstrap-guidelines` task and `.trellis/spec/`. Slice 2 (`05-16-02-local-render`, archived) shipped the renderer that produces `bundles/<profile>/<version>/{manifest.json,checksums.json,files/...}`. Slice 3 reads that output and writes to disk.

## Scope

### In scope

- `confighub apply --bundle <path>` — main flow. Defaults: dry-run-style diff displayed then prompts for confirmation unless `--yes` is given.
- `confighub diff --bundle <path>` — read-only diff between bundle and local targets.
- `confighub status` — print active profile, last applied bundle version, last apply timestamp, last backup directory.
- `confighub rollback` — restore the last backup for the active profile.
- `confighub doctor:apply` — write-path health checks (disk space, target dir writability, backup dir creatability).
- Per-profile apply lock at `state/apply.lock` (PID file). Live PID -> exit 13; stale PID -> exit 13 + clear "run lock release --force" instruction; no auto-clear.
- Target path policy (see "Target Path Policy" below). Validation runs before any backup or write.
- Symlink handling: default reject; honor `safety.symlink: replace` (replace the symlink with a regular file) or `follow` (write through the symlink, but reject if it points outside the policy).
- `removedFiles` handling: back up the existing local file (if present), refuse if local checksum does not match `previousChecksum`, then delete. A missing local file is a no-op, not an error.
- Managed-section / `includeStrategy: append-once`: search target file for the marker block `# >>> confighub:<templateId> >>>` ... `# <<< confighub:<templateId> <<<`. If absent, append marker block + rendered content. If present, replace the content between markers, leaving the rest of the file untouched. Marker comment style follows the table in `.trellis/spec/shared/config-safety.md` ("Marker Block Format"). JSON files cannot use marker blocks — a JSON template with `merge: managed-section` is a validation error in Slice 3.
- Backup directory: `~/.confighub/backups/<bundleVersion>/`, mode `0700`. Per-file backups inside, mode `0600`. The bundle's manifest entry's `targetPath` is mapped to a path under the backup dir preserving the home-relative structure (e.g. `~/.gitconfig` -> `backups/<version>/dotfiles/gitconfig.bak`). Document the exact mapping in code comments.
- Atomic write: write to a sibling temp file in the same directory, fsync, `os.Rename`, fsync directory. Failure mid-write removes the temp file.
- Apply log at `state/apply.log` (newline-delimited JSON). Entry shape documented below. Apply log redacts secret-derived content (records checksum + target path + action, never bytes).
- Rollback pointer at `state/rollback/<profileId>.json` records the most recent successful apply's backup directory.

### Out of scope

- Pulling bundles from a hub — Slice 5.
- Serving — Slice 4.
- Bundle signature verification — Slice 5.
- Multi-profile concurrent apply — single operator + apply.lock is enough.
- Conflict-resolution UI — backlog.
- `deep-merge` strategy for JSON/TOML/YAML — reserved per `config-safety.md`. Slice 3 supports `replace` and `managed-section` only; encountering `deep-merge` errors with a "not implemented before Slice 4" message at the moment.

## CLI

```
confighub apply    --bundle <path> [--profile <id-or-path>] [--root <dir>] [--yes] [--dry-run] [--json]
confighub diff     --bundle <path> [--profile <id-or-path>] [--root <dir>] [--json]
confighub status   [--profile <id-or-path>] [--root <dir>] [--json]
confighub rollback [--profile <id-or-path>] [--root <dir>] [--yes]
confighub doctor:apply [--profile <id-or-path>] [--root <dir>]
```

- `--bundle <path>` is required for `apply` and `diff`. Resolution: if the value contains `/`, treat as a path. Otherwise look up under `<root>/bundles/<active-profile>/<value>/`. Special case: `--bundle latest` resolves to the lexicographically last version for the active profile.
- `--profile` resolution: same logic as Slice 2 (path if contains `/` or `.yaml`; else id under `<root>/profiles/<id>.yaml`). Defaults to the value in `state/active-profile` if no flag is given. If neither is set, apply/diff/status/rollback all exit `2` with "no active profile".
- `--yes` skips the interactive diff-then-confirm step on `apply` and `rollback`.
- `--dry-run` on `apply` performs the full pipeline up to and including diff computation but skips all writes (no backup, no apply log update).
- Exit codes (per `.trellis/spec/backend/cli.md`):
  - `0` success.
  - `2` usage / missing inputs.
  - `10` validation error (bundle invalid, manifest broken, schema mismatch).
  - `11` target path policy rejection (symlink, forbidden root, etc.).
  - `12` checksum verification failure.
  - `13` lock acquisition failure (apply already running).
  - `30` rollback failure (backup unreadable or atomic write failed during rollback).

## Target Path Policy

Each `targetPath` (after `~` expansion via `os.UserHomeDir()`) must be inside one of these allowlisted roots:

- `$HOME/.confighub/...` (ConfigHub's own state and backups)
- `$HOME/.config/...` (XDG-style)
- `$HOME/.codex/...`
- `$HOME/.claude/...`
- `$HOME/.local/share/...`

Plus this allowlist of file names directly under `$HOME` (no deeper, no globbing):

- `.gitconfig`
- `.zshrc`
- `.zshenv`
- `.bashrc`
- `.bash_profile`
- `.profile`
- `.tmux.conf`
- `.vimrc`

Forbidden (reject with exit 11 even if inside an allowlisted root):

- Anything under `$HOME/.ssh/` (private keys, known_hosts)
- Anything under `$HOME/.gnupg/`
- Anything matching `.*history`, `.*_history`, or `.cache`
- Anything matching `*.sqlite`, `*.db`, `*.kdbx`

Path normalization: resolve `~`, clean (`filepath.Clean`), reject if the result contains `..` segments or escapes `$HOME`. Reject absolute paths outside `$HOME`.

## Apply Algorithm (canonical order)

1. Acquire `state/apply.lock` (per current profile). Fail fast on contention.
2. Load bundle: read `manifest.json`, validate schema, load `checksums.json`.
3. For every `files[]` entry: read the rendered file from `<bundle>/files/<bundlePath>`, recompute sha256, compare to manifest checksum and checksums.json entry. Mismatch -> exit 12.
4. For every `files[]` and `removedFiles[]` entry: normalize and policy-check the target path. Reject -> exit 11.
5. Compute a per-file action:
   - `wrote` if target is missing or differs from rendered content.
   - `unchanged` if target equals rendered content.
   - `managed-section-update` if `safety.merge == managed-section`.
   - For `includeStrategy: append-once`: `included-once` if marker absent, `unchanged` if marker present and content matches, `managed-section-update` if marker present and content differs.
6. For `removedFiles[]`: verify local checksum matches `previousChecksum` (exit 11 if not). Action: `removed`. Missing local file -> action `removed-noop`.
7. Render and print the diff. If interactive and not `--yes`, prompt to confirm.
8. Create backup dir `~/.confighub/backups/<bundleVersion>/`, mode `0700`. For every file that will be written (action != `unchanged`/`removed-noop`), copy the existing target to the backup dir preserving relative path and original mode (default `0600` if not present).
9. Write each file atomically (temp file in target dir, fsync, rename, fsync parent dir). Honor symlink policy: default reject; `replace` removes symlink first then writes; `follow` resolves and writes through the link only if the resolved path is still in policy.
10. Delete `removedFiles[]` targets after backup.
11. Write rollback pointer at `state/rollback/<profileId>.json` -> `{"backupDir": "<absolute path>", "bundleVersion": "...", "appliedAt": "..."}`.
12. Append a new entry to `state/apply.log`.
13. Release `state/apply.lock`.

Any failure between steps 8 and 13 leaves a complete backup behind so manual or `confighub rollback` recovery is always possible.

## Apply Log Entry Shape

```json
{
  "appliedAt": "2026-05-16T16:42:10Z",
  "profileId": "macbook",
  "bundleVersion": "2026-05-16T16-42-10Z-001",
  "backupDir": "/Users/ruichen/.confighub/backups/2026-05-16T16-42-10Z-001",
  "files": [
    {"templateId": "ai/claude", "targetPath": "/Users/ruichen/.claude/settings.json", "checksum": "sha256:...", "action": "wrote", "previousChecksum": "sha256:..."},
    {"templateId": "dotfiles/git-include", "targetPath": "/Users/ruichen/.gitconfig", "checksum": "sha256:...", "action": "managed-section-update", "previousChecksum": "sha256:..."}
  ],
  "removedFiles": []
}
```

- All paths are absolute (post `~` expansion).
- `previousChecksum` is `null` if the target did not exist before apply.
- `action` is one of: `wrote`, `unchanged`, `managed-section-update`, `included-once`, `removed`, `removed-noop`.
- The entry never contains rendered file content or secret material.

## Marker Block Format (for managed-section)

Comment style per file format (matches `config-safety.md`):

- Shell, Git config, TOML, gitconfig, sshconfig (when added later): `# >>> confighub:<templateId> >>>` open, `# <<< confighub:<templateId> <<<` close. Template id is kebab-case; replace `/` with `-` for the marker (e.g. `dotfiles/git-include` -> `dotfiles-git-include`).
- JSON: not supported in Slice 3 (validation error).
- Other formats: rejected in Slice 3 with "marker style not registered for this file type" message; future slices can add.

Apply rules:

- Locate the open marker line and matching close marker line in the existing target file. Lines outside that range are preserved byte-for-byte.
- Replace the content between markers (exclusive of marker lines themselves) with the rendered content, preserving the original line ending.
- If no marker pair found, append marker pair + content to the end of the file.
- If multiple marker pairs with the same id are found, fail with exit 10.

## Required Additions / Updates to Existing Code

- `internal/apply/` — new files: `apply.go`, `diff.go`, `backup.go`, `markers.go`, `policy.go`, `log.go`, `rollback.go`, `status.go`, `doctor.go`, plus tests.
- `internal/apply/lock.go` — per-profile apply lock, mirror of `internal/render/lock.go`. Or reuse a shared `internal/lockfile/` package — pick one and stay consistent.
- `cmd/confighub/apply.go`, `diff.go`, `status.go`, `rollback.go`, `doctor.go` — replace Slice 1 stubs.
- `internal/bundle/` — extend `LoadManifest` callers to also load `checksums.json` and verify all `files[].checksum`. May add `LoadBundle(path string) (*Manifest, files map[string][]byte, err error)` helper.

## Acceptance Criteria

- [ ] `confighub apply --bundle <slice-2-rendered-bundle> --profile macbook --yes` against a `t.TempDir()` HOME successfully writes the three rendered files, backs up any pre-existing files, and produces an apply log entry.
- [ ] Re-applying the same bundle is a no-op: every action is `unchanged`, no backup files are created.
- [ ] `confighub diff --bundle <path>` shows a unified diff for each file that differs and "no change" for the rest, exits 0.
- [ ] `confighub status` prints active profile, last bundle version, last apply timestamp, last backup directory, exits 0.
- [ ] `confighub rollback --yes` restores the previous file contents byte-for-byte for every file that was written by the last apply. Exit 0.
- [ ] A manifest entry with `targetPath: ~/.ssh/config` is rejected with exit 11 even if the target is technically inside `$HOME`.
- [ ] An entry whose `bundlePath` rendered file checksum differs from the manifest entry checksum is rejected with exit 12 (no writes occur).
- [ ] A target that is a symlink is rejected with exit 11 by default; if the template declares `safety.symlink: replace`, the symlink is removed and a regular file is written in its place.
- [ ] A `removedFiles[]` entry whose local file checksum does not match `previousChecksum` is rejected with exit 11 and no deletions occur.
- [ ] A `removedFiles[]` entry whose local file does not exist is silently a no-op (`removed-noop`).
- [ ] `includeStrategy: append-once` correctly inserts the marker block on first apply and is idempotent on second apply; manually edited content above and below the marker block survives both apply and rollback.
- [ ] `apply.log` exists, is newline-delimited JSON, each entry parses, contains no secret-derived bytes. A `secret` keyword search across the log returns nothing other than declared field names.
- [ ] Two concurrent `confighub apply` for the same profile: second exits 13.
- [ ] All Slice 2 tests still pass.
- [ ] `go test ./...` passes; new tests cover all of the above (use `t.TempDir()` for every test that touches disk).
- [ ] `go vet ./...` clean.

## References

- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/implement.md` — Slice 3 detailed checklist.
- `.trellis/tasks/archive/2026-05/00-bootstrap-guidelines/design.md` — apply rules and safety model.
- `.trellis/spec/backend/apply-engine.md` — required flow, test classes.
- `.trellis/spec/shared/config-safety.md` — target path policy, marker block format, merge strategies.
- `.trellis/spec/shared/concurrency.md` — atomic write pattern, lock semantics.
- `.trellis/spec/shared/bundle-contract.md` — manifest schema (especially `removedFiles`).
- `.trellis/spec/backend/cli.md` — flag and exit-code conventions.
- `.trellis/spec/backend/state-directory.md` — apply.log and rollback pointer layout.

## Notes for the Implementer (Pi)

- Do not commit. Leave changes uncommitted.
- Touch only files needed for Slice 3:
  - `cmd/confighub/*.go` — replace Slice 1 stubs for apply, diff, status, rollback, doctor.
  - `internal/apply/*.go` — the bulk of the new code.
  - `internal/bundle/*.go` — small additions for full bundle loading. Do not break Slice 1/2 types.
- Do not modify `.trellis/`, `.claude/`, `.codex/`, `.agents/`, `README.md`, `AGENTS.md`, fixtures, or other slices' work.
- Use stdlib for diff output (you can adapt a simple Myers-diff or use `golang.org/x/text/diff` — if you add a new dep, pin it and document why). For Slice 3 a simple line-based diff is fine; full unified-diff format is not required as long as `confighub diff` output is readable.
- Reuse the Slice 2 render output as the test bundle: run render in a `t.TempDir()` test setup, then apply, then verify.
- When unsure, leave `// TODO(slice-3): <question>` and continue.

## Reporting

When done, print:

1. List of files created / modified (path only).
2. `go test ./...` output (summary line per package).
3. `go vet ./...` output (or "clean").
4. A sample run output (sanitized): render a bundle in a temp dir, apply it, show the apply log entry that was produced.
5. Any TODO comments you left, with file:line.
6. Any decision you made that wasn't covered in the spec.
