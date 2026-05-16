# Concurrency Model

ConfigHub MVP runs under a **single operator, single hub-writer** assumption. This document spells out what that means and how the renderer/server/apply engine cooperate without a database.

## Hub-Side Rules

- Only one process (the operator) writes to `profiles/` and `templates/` at a time. Multi-user concurrent writes are out of MVP scope.
- The renderer writes into `bundles/.tmp/<bundleVersion>/`, then performs `os.Rename` into `bundles/<profileId>/<bundleVersion>/`. Concurrent readers see either the previous bundle or the new one, never a partial tree.
- The renderer acquires `state/render.lock` (PID-bearing advisory file). A second renderer that finds an existing lock with a live PID exits non-zero with a clear message. A stale lock (process gone) requires explicit operator action; the renderer does not auto-delete it.
- `confighub serve` reads from `bundles/<profile>/<version>/` and `profiles/`. Reads use file-handle semantics that survive atomic rename of the directory.

## Client-Side Rules

- `confighub apply` acquires `state/apply.lock` per profile before any backup or write. A second apply on the same profile refuses to start.
- Backups under `~/.confighub/backups/<timestamp>/` are timestamped, so two simultaneous applies (if a lock is bypassed) cannot clobber each other's backups.
- `confighub pull` writes into `state/pull/.tmp/<profile>/`, then renames into `state/pull/<profile>/` once verification (manifest + checksums + signature) succeeds.

## Atomic Write Pattern (canonical)

Every persistent write follows this pattern:

```text
1. Create temp file in the same filesystem (same parent dir).
2. Write content + fsync.
3. Rename temp -> final path.
4. fsync parent directory.
```

Tests must cover:

- crash between step 2 and step 3 (no half-final file).
- crash between step 3 and step 4 (final file present but parent dir entry not durable — acceptable; we re-verify on next run).
- concurrent reader during step 3 (reader sees either old or new content, never partial).

## What This Does Not Cover

- Multi-operator hubs.
- Distributed deployments.
- Network-filesystem-backed storage where rename atomicity may not hold (NFS, SMB). MVP recommends local disk only.

If a future requirement breaks any of these assumptions, this spec must be updated before code changes.
