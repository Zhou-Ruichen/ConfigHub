# HTTP API Guidelines

## MVP Routes

- `GET /`
- `GET /api/v1/status`
- `GET /api/v1/profiles`
- `GET /api/v1/profiles/{id}`
- `GET /api/v1/profiles/{id}/bundle`
- `GET /api/v1/profiles/{id}/bundle/manifest`
- `GET /api/v1/profiles/{id}/bundle/signature`
- `GET /api/v1/profiles/{id}/templates/{templateId}`

## Response Rules

- JSON API responses use stable field names. Breaking schema changes bump `schemaVersion` in the response body.
- Errors follow `{ "code": "<machine-code>", "message": "<human-readable>" }` and a meaningful HTTP status.
- HTML routes render usefully without client-side JavaScript. Copy-to-clipboard and diff toggle may use minimal embedded JS.
- Direct template routes must check `delivery.remote` and authorization scope before serving bytes.
- Secret-derived values are redacted unless the template **and** the requester explicitly permit access. See [../big-question/secret-handling.md](../big-question/secret-handling.md).

## Direct Read Response Contract

`GET /api/v1/profiles/{id}/templates/{templateId}` for a `delivery.remote` template:

- `200 OK`:
  - Body: raw rendered bytes (no JSON wrapping).
  - `Content-Type`: value declared on the template entry; default `application/octet-stream`.
  - `ETag`: `"<sha256-hex>"` of the rendered bytes (matches `checksum` in the manifest).
  - `Last-Modified`: RFC1123 timestamp of the bundle's `createdAt`.
  - `X-ConfigHub-Profile`: `<profileId>`.
  - `X-ConfigHub-Bundle`: `<bundleVersion>`.
- `304 Not Modified`: returned when the request includes `If-None-Match: "<sha256-hex>"` and the rendered bytes are unchanged.
- `404 Not Found`: returned when the template does not exist, is not declared `delivery.remote`, **or** the requester is unauthorized for a secret-bearing template. Existence of secret-bearing templates is never leaked through `401`/`403`.
- `406 Not Acceptable`: returned if `Accept` is set and excludes the template's `Content-Type`.
- `429 Too Many Requests`: optional, when a future rate-limit is added.

Bundle archive responses (`/profiles/{id}/bundle`) include the secret-bearing file only when the requester is authorized; otherwise the archive omits those entries and the response manifest reports them under `redactedFiles` (not `files`).

## Authentication

- Loopback binds (`127.0.0.1`, `::1`) may start permissive in development.
- Any non-loopback bind **must** be configured with at least one token before it accepts traffic; startup exits non-zero otherwise.
- Tokens travel in `Authorization: Bearer <token>`.
- Tokens are bound to scopes (`pull:<profileId>`, `read:<templateId>`, `admin`); the server consults the scope before serving a route.
- Token plaintext is produced once at creation, returned to the operator, and never persisted. Only the hash is stored under `state/tokens/<id>.json`.
- Bearer tokens and rendered secret values must never appear in any log line.

## TLS

- Non-loopback binds should serve via TLS, either built-in or behind a reverse proxy.
- Built-in TLS is optional in MVP; the server logs a warning on startup if a non-loopback bind starts without TLS.
- Reverse-proxy examples land in `ops/` during Slice 8.

## Caching

- All bundle and template responses expose `ETag` headers matching the manifest checksum.
- Clients are expected to send `If-None-Match` to skip re-downloading unchanged content.
- The hub may emit `Cache-Control: no-cache` to force revalidation while still allowing 304 responses.
