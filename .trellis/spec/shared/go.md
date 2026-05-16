# Go Guidelines

## Project Layout

Use a single Go module with one command:

```text
cmd/confighub/
internal/profile/
internal/template/
internal/render/
internal/bundle/
internal/apply/
internal/server/
internal/web/
internal/secret/
internal/domain/
```

`cmd/confighub` wires commands and dependencies. Business logic lives under `internal/`.

## Dependency Policy

- Prefer the standard library.
- Add dependencies only when they replace real complexity.
- Keep dependencies small and auditable because `confighub` is a configuration-writing tool.
- Do not add a database dependency for MVP storage.

## Error Handling

- Return errors with context using `fmt.Errorf("...: %w", err)`.
- Do not panic for user/config/template errors.
- CLI commands should produce concise actionable messages.
- Server handlers should return structured JSON errors for API requests and readable HTML errors for web UI requests.

## Filesystem Rules

- Use explicit path normalization before policy checks.
- Do not write outside allowlisted target roots.
- Write to a temp file in the target directory, then atomically rename.
- Preserve configured file modes.
- Treat symlinks according to the template safety policy; reject by default.

## Testing

- Use table-driven tests for profile/template/bundle validation.
- Use `t.TempDir()` for render/apply tests.
- Write apply-engine tests before enabling real home-directory targets.
- Test both success and rejection paths for unsafe templates.
