# State Directory

`state/` is the on-disk home for runtime metadata, advisory locks, and per-machine pointers. It is **not** a database, but it is the most security-sensitive directory in the project after `bundles/` and rendered targets.

## Layout

```text
state/
  apply.log              # newline-delimited JSON; one entry per applied bundle
  apply.log.1            # most-recent rotation (kept; older rotations purged)
  pull/<profile>.json    # last pulled bundle metadata for this client
  pull/.tmp/<profile>/   # in-progress pull staging (atomic-rename target)
  tokens/<id>.json       # token metadata: id, label, scope, hash, createdAt; no plaintext
  render.lock            # hub-side advisory PID file for active renders
  apply.lock             # client-side advisory PID file for active applies (per profile)
  active-profile         # optional pointer file: a single profile id
```

## File Modes and Ownership

- Every file under `state/` is created with mode `0600`.
- `state/` itself is `0700`.
- On the hub, ownership is the operator account that runs `confighub serve`.
- On a client, ownership is the user running `confighub apply`.

## What Must Never Appear in `state/`

- Plaintext tokens.
- Rendered secret values.
- Full bundle content (those live in `bundles/`).
- User home-directory contents (those live in `~/.confighub/backups/...`).

Tests must include negative assertions: load each `state/` file produced by the test fixtures and assert that no token-like or secret-like string appears.

## Rotation and Retention

- `apply.log` rotates when it exceeds 10 MiB. One rotated file is kept (`apply.log.1`).
- `tokens/<id>.json` is removed only when an operator runs `confighub token revoke <id>`.
- `pull/<profile>.json` is overwritten on each successful pull.
- Stale `render.lock` / `apply.lock` files are not auto-purged; the operator must run `confighub lock release --force` after confirming no process holds the lock.

## Atomic Writes

Every write to `state/` follows the temp-file + rename pattern documented in [shared/concurrency.md](../shared/concurrency.md). Partial writes are never observable to readers.
