# Secret Handling

This guide governs how ConfigHub treats configuration that may carry secret-derived values (API keys, OAuth tokens, MCP credentials). The AI domain triggers this guide most often.

## Where Secrets May Appear

- AI provider settings (Codex, Claude, OpenCode, Gemini).
- MCP endpoint credentials.
- Provider-gateway shared secrets.

Anything that ends up inside a config file the operator would otherwise gitignore is a secret-derived value here.

## Template Declaration

A template that may render a secret value must declare both:

- `safety.secrets: allowed`
- A merge strategy that does not silently echo the secret elsewhere (see [shared/config-safety.md](../shared/config-safety.md) "Settings Merge Strategy").

Templates default to `safety.secrets: forbidden`. The renderer refuses to inline secret-bearing references unless the template opts in.

## Storage at Rest

- Bundles containing secret-bearing files live on the operator's local disk under `bundles/`. Filesystem mode is `0700` on `bundles/`; per-file mode is `0600` unless the template requires otherwise.
- ConfigHub does not encrypt bundle files in MVP. The operator relies on disk encryption (FileVault, LUKS, full-disk-encryption on the VPS) and access control to protect the hub.
- Local secret references (`age`-encrypted files under `~/.config/confighub/secrets/<profile>.age`) and environment variables are the supported secret sources for MVP.

## Transport

- Loopback binds may serve secret-bearing bundles without TLS for local development only.
- **Any non-loopback bind serving a `secrets: allowed` template requires TLS** (built-in or via reverse proxy). `confighub serve` warns on startup if a non-loopback bind is HTTP.
- Pull and direct-read requests for secret-bearing templates require `Authorization: Bearer <token>`.
- Tokens are bound to a scope (`pull:<profileId>`, `read:<templateId>`). Reads outside scope return `404` — never `401` — to avoid disclosing template existence.

## API Behavior

- `GET /api/v1/profiles/{id}/templates/{templateId}` for a secret-bearing template:
  - returns `404` if the requester is unauthenticated.
  - returns `404` if the requester is authenticated but lacks the right scope.
  - returns `200` with the rendered bytes if the template declares `delivery.remote` and the requester is in scope.
- Bundle archive responses (`/profiles/{id}/bundle`) include the secret-bearing file only when the requester is authorized; otherwise the archive omits those entries and the manifest reports them under `redactedFiles`.

## Logging and Metadata

- Apply logs record file path, checksum, target, timestamp, and bundle version — never the rendered secret value.
- Audit logs on the hub record requester token id, requested template id, response status — never the rendered value.
- Error messages must not echo template content. Errors include the template id and a generic reason ("forbidden", "checksum mismatch") instead.

## Backups

- Backups under `~/.confighub/backups/<timestamp>/` may legitimately contain previous secret-bearing files. Backup directory mode is `0700`; per-file mode is `0600`.
- Retention policy (Slice 8) deletes backups by count and age, never by content inspection — but old backups containing former secrets remain readable to the operator until purged.

## When This Guide Triggers a Review

- Any new template that may render a secret value.
- Any change to `delivery.remote` for an existing template.
- Any change to token scope semantics.
- Any change to apply-log or audit-log format that affects what fields are recorded.
