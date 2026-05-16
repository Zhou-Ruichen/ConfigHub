# Security Guidelines

## Access Model

ConfigHub may be reachable from multiple devices owned by the single operator. Treat all non-local access as untrusted until authenticated.

Rules (in force from Slice 4 onward):

- Loopback (`127.0.0.1`, `::1`) binds may run permissive in development only.
- Any non-loopback bind requires at least one token configured before startup. The server exits non-zero if a non-loopback bind starts without a token.
- Tokens travel via `Authorization: Bearer <token>`.
- Token authenticity is checked via constant-time compare against the stored hash; plaintext tokens are never persisted (see [state-directory.md](./state-directory.md)).
- No token plaintext or rendered secret value appears in any log line.
- API metadata redacts secret-derived values unless the requester is authorized for that template.
- Client apply still verifies pulled bundles (manifest + checksums; signature from Slice 5).

## Secret Handling

Detailed flow lives in [../big-question/secret-handling.md](../big-question/secret-handling.md). Quick rules:

- A template that may render a secret value must declare `safety.secrets: allowed`.
- Bundles containing secret-bearing files are not delivered over unauthenticated transport.
- Direct remote reads of secret-bearing templates require both `delivery: remote` opt-in and an authenticated, in-scope requester.
- Unauthenticated requests for secret-bearing templates return `404`, not `401`, so existence is not leaked.
- Apply and audit logs record file path, checksum, target, timestamp, bundle version — never the rendered value.

## TLS

- Non-loopback binds should run behind TLS (reverse proxy or built-in).
- Built-in TLS is optional; the server logs a startup warning when a non-loopback bind starts without TLS.
- Reverse-proxy examples (Caddy, nginx) ship under `ops/` during Slice 8.

## Dangerous Templates

Templates targeting sensitive paths must be rejected unless a project spec explicitly allows them.

Forbidden by default (also enumerated in [../shared/config-safety.md](../shared/config-safety.md)):

- private keys;
- `known_hosts`;
- session files;
- history files;
- runtime databases;
- caches;
- logs;
- GUI state.

## Audit Trail

Record:

- profile id;
- bundle version;
- requester token id (when available);
- template ids;
- target paths;
- checksums;
- timestamps (RFC3339 UTC).

Do not record rendered secret values, token plaintext, or any user-supplied content.
