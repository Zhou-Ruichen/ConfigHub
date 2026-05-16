# Service Architecture

## Command Model

`confighub` is one binary with multiple modes (subcommand naming, flags, and exit codes are documented in [cli.md](./cli.md)):

- `serve`: web UI and HTTP API.
- `render`: render bundles from local profiles/templates.
- `init`: configure a client against a hub.
- `pull`: fetch bundle metadata/files.
- `diff`: compare bundle output with local targets.
- `apply`: backup and write files.
- `rollback`: restore previous backup.
- `doctor:apply` / `doctor:tools`: health checks for write path and installed tools.

## Package Boundaries

- `profile`: profile parsing and validation.
- `template`: template definitions and delivery/safety metadata.
- `render`: deterministic rendering from profile/template inputs.
- `bundle`: manifest, checksum, bundle read/write, `removedFiles` semantics.
- `apply`: local filesystem diff/backup/write/rollback, marker-block handling, `includeStrategy` execution.
- `server`: HTTP routes and API behavior.
- `web`: server-rendered UI.
- `secret`: local secret references first, encrypted-file references later, external adapters last.
- `domain`: AI and dotfiles domain-specific targets.

Do not let `server` or `web` write target files directly. Local writes belong to `apply`.

## Storage

MVP storage is a directory tree:

```text
profiles/
templates/
bundles/
  .tmp/<bundleVersion>/    # in-progress renders (atomic-rename source)
  <profileId>/<bundleVersion>/
state/
  apply.log
  pull/<profile>.json
  pull/.tmp/<profile>/
  tokens/<id>.json
  render.lock
  apply.lock
  active-profile
```

- See [state-directory.md](./state-directory.md) for file modes, retention, and forbidden contents.
- See [../shared/concurrency.md](../shared/concurrency.md) for the single-writer assumption and atomic-rename rules.
- Do not introduce a database until file-backed storage fails a documented requirement.

## Determinism

Rendering is deterministic for the same inputs, except for the explicit metadata fields `createdAt` and `bundleVersion`. Two consecutive `confighub render` runs with identical inputs must produce identical files and identical checksums in `files[]`.
