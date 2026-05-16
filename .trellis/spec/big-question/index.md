# Common Design Questions

Use these guides before changing configuration distribution, template boundaries, bundle contracts, or local apply behavior.

## Issue Index

| Issue | Category | Severity |
| --- | --- | --- |
| [Config Distribution](./config-distribution.md) | Architecture | Critical |
| [Template Boundaries](./template-boundaries.md) | Safety | Critical |
| [Secret Handling](./secret-handling.md) | Safety / Security | Critical |
| [System Constraints](./system-constraints.md) | Product Design | Warning |

## Quick Checklist

Before adding a template domain:

- [ ] Is the source template safe to distribute?
- [ ] Is the target path allowlisted?
- [ ] Does it contain private/local/runtime state?
- [ ] Is delivery `sync`, `remote`, or both?
- [ ] Is the merge strategy declared (`replace` | `managed-section` | `deep-merge`)?
- [ ] Does apply require backup and diff?
- [ ] If secret-bearing: is `safety.secrets: allowed` declared and does the transport require TLS + token?

Before changing pull/apply:

- [ ] Are manifests verified?
- [ ] Are checksums verified?
- [ ] Are signatures verified (Slice 5+)?
- [ ] Are symlinks handled explicitly?
- [ ] Is rollback still possible?
- [ ] Are secrets redacted from logs and metadata?
- [ ] Is `removedFiles` processed (back up, then delete)?

Before adding external services:

- [ ] Can the MVP file-backed model solve this first?
- [ ] Is the dependency optional?
- [ ] Is there a narrow adapter interface?
