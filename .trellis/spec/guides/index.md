# Thinking Guides

ConfigHub's primary thinking guides live in package-specific spec directories. Use them in this order:

- [Config Distribution](../big-question/config-distribution.md) — sync vs. remote delivery decisions.
- [Template Boundaries](../big-question/template-boundaries.md) — what may be templated and what may not.
- [Secret Handling](../big-question/secret-handling.md) — how secret-bearing templates are stored, transported, and read.
- [System Constraints](../big-question/system-constraints.md) — MVP non-negotiables.
- [Bundle Contract](../shared/bundle-contract.md) — manifest, checksum, lifecycle semantics.
- [Configuration Safety](../shared/config-safety.md) — forbidden paths, merge strategy, apply safety.
- [Concurrency Model](../shared/concurrency.md) — single-writer hub, atomic-rename rules.
- [State Directory](../backend/state-directory.md) — what lives in `state/` and how it is protected.
- [CLI Ergonomics](../backend/cli.md) — subcommand, flag, and exit-code conventions.

## Pre-Modification Rule

Before changing a template id, target path, manifest field, API route, or apply behavior, search for existing references first:

```bash
rg "value_to_change"
```

Then update README, task artifacts, specs, tests, and examples together.

## Core Principles

1. Config is data with safety policy, not arbitrary files.
2. A reachable hub is convenient, but clients still verify pulled bundles (manifest + checksums + signature from Slice 5).
3. Local writes require diff, backup, atomic write, and rollback.
4. Remote reads require explicit delivery permission and authorization.
5. Optional adapters must not become hidden MVP requirements.
6. The hub assumes a single operator. Multi-user concerns are out of MVP scope; do not design around them.
