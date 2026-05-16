# Backend Quality Guidelines

## Required Checks

- `go test ./...`
- `go vet ./...`

For apply-engine changes, include temporary-directory tests that prove no real home path is required.

## Review Checklist

- [ ] Package boundary is clear.
- [ ] File-backed storage remains deterministic.
- [ ] API handlers do not bypass renderer/bundle/apply contracts.
- [ ] Pull/apply verifies manifests and checksums.
- [ ] Target paths are normalized before policy checks.
- [ ] Symlink behavior is explicit.
- [ ] Secrets and tokens are redacted from logs.
- [ ] Errors are actionable for CLI users.
- [ ] Web/API behavior remains usable without mandatory external services.

## Dependency Review

Before adding a dependency, document:

- what complexity it removes;
- why the standard library is insufficient;
- whether it increases risk for a configuration-writing tool.
