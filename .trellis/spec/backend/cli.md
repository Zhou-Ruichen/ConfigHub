# CLI Ergonomics

ConfigHub is a CLI-first product. These rules apply to every command exposed by `confighub`.

## Subcommand Naming

- Use lowercase, hyphenated subcommands. Example: `confighub apply`, `confighub doctor:apply`.
- Use a colon namespace (`doctor:apply`, `doctor:tools`) when one verb fans out into per-domain variants.
- Avoid abbreviations except where the underlying tool is well-known (e.g. `init`).
- Reserve `serve` for hub mode. CLI verbs operate on local state unless they explicitly require a hub URL.

## Standard Flags

- `--profile <id>`: required when a command operates on a profile and there is no `state/active-profile`.
- `--bundle <path>`: required when a command operates on an already-rendered bundle.
- `--from <url>`: hub URL for `init`, `pull`.
- `--token <token>` or `CONFIGHUB_TOKEN` env: token for non-loopback hub access.
- `--root <dir>`: override the hub root directory for `serve` (default `./`).
- `--bind <addr>`: bind for `serve` (default `127.0.0.1:8787`; non-loopback binds require a configured token).
- `--dry-run`: never writes. Required for any command that would otherwise mutate state.
- `--yes`: disables interactive confirmation for diff -> apply. Must not be the default.
- `--json`: machine-readable output. Stable schema documented per command.
- `--quiet`: suppress non-error output.
- `--verbose`: include debug detail, never secrets.

## Exit Codes

- `0`: success.
- `1`: generic failure.
- `2`: usage error (missing or invalid flags).
- `10`: validation error (profile, template, or manifest invalid).
- `11`: target path policy rejection.
- `12`: checksum or signature verification failure.
- `13`: lock acquisition failure (another renderer or apply is active).
- `20`: network or hub error.
- `21`: authentication error (token missing, invalid, or rejected).
- `30`: rollback failure (backup unreadable or write failed).

Tests must assert exit codes, not just `non-zero`, for any failure mode the operator may script against.

## Output Conventions

- Human-readable output is the default. Use color only on TTY; respect `NO_COLOR`.
- `--json` output is one JSON object per top-level result (no JSON Lines for single-result commands; JSON Lines is acceptable for streaming subcommands such as `diff`).
- Never print rendered secret values, token plaintext, or backup file content.
- Diff output truncates lines longer than 4 KiB with an explicit truncation marker.
- All times are RFC3339 UTC unless the operator is on a TTY and `--verbose` opts into local time.

## Help Text

- Every subcommand has a one-line summary and an example.
- Examples use `~/.config/confighub/...` or `examples/...` paths only. No real home directories in help text.

## Compatibility

- New flags must be additive. Removing or renaming a flag requires a deprecation cycle: print a warning for one release, then remove.
- `--json` schemas are versioned. Breaking changes bump the schema version embedded in the response.
