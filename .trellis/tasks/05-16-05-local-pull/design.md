# Slice 5 Design — Pull flow + ed25519 signatures

## Architecture

Two new concerns layered on top of Slices 3–4:

1. **Server-side signing**: the running hub holds an ed25519 keypair under `state/signing-key.json`. Every manifest served by `/api/v1/profiles/<id>/bundle/manifest` is signed at serve time. The public key is exposed via `/api/v1/signing-key`.
2. **Client-side pulling**: a new `confighub init` / `pull` / `apply --from-pulled` chain that fetches bundles from a hub URL, verifies signature + checksums + schema, caches them under `state/pull/<profile>/<bundleVersion>/`, and feeds the existing Slice 3 apply engine through a `latest` symlink.

No changes to the render path itself other than appending a `signature` field to the manifest after rendering.

## Decisions

### Signing algorithm: ed25519

- 32-byte private key, 32-byte public key, 64-byte signature. Fast, deterministic, no parameter selection.
- Go stdlib `crypto/ed25519` covers generate / sign / verify; no external deps.
- Signatures are deterministic, so the same manifest signed twice produces the same signature. Useful for diffing and reproducibility.

### Sign at serve, not at render

Two reasons:

1. Render output is the source of truth on disk; rewriting it to inject a signature is fragile and would invalidate already-checksummed render output.
2. Each hub instance has its own key. Render output can be moved between hubs (e.g., dev → prod) without re-signing being a render-time concern.

Concretely: the render command writes `manifest.json` with `signature: null` (or `signature` field absent). The serve handler reads it, computes the signature, and returns the augmented manifest.

### Manifest canonicalization

Signature is over the manifest JSON with these rules:

1. Set `signature` field to `nil` before serializing.
2. Marshal with `json.Marshal` after sorting struct fields stably. Since Go's `encoding/json` for structs already emits a stable order (declaration order), we just need to ensure no `map[string]any` with non-deterministic iteration leaks in. Current manifest has none.
3. Do not html-escape: use `json.Encoder` with `SetEscapeHTML(false)`.
4. No trailing newline.

The same canonicalization is used on verify. A small `internal/sign/canonical.go` wraps this so both sides call the same code path.

### Signing key lifecycle

On `confighub serve` startup:

```go
keyPath := filepath.Join(stateDir, "signing-key.json")
if _, err := os.Stat(keyPath); os.IsNotExist(err) {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    writeKeyFile(keyPath, pub, priv, time.Now().UTC())
}
load(keyPath) // mandatory
```

`signing-key.json` shape:

```json
{
  "algorithm": "ed25519",
  "publicKey":  "<base64>",
  "privateKey": "<base64>",
  "createdAt":  "2026-05-16T20:00:00Z"
}
```

Mode 0600. Created by the server process; under Docker, that's UID 65532. The mounted `/state` volume on hk-cn2 already has the right ownership from earlier slices.

### Client pinning

After `confighub init`, the local hub config (`state/hub.json`) stores the public key. Every subsequent pull verifies against the stored key. If the server's key changes (rotation, new hub instance), pull rejects with `pinned public key mismatch` and prints the new fingerprint + a remediation hint pointing to `--accept-new-key`. The accept-new-key flow is **not implemented this slice** — we only need to detect and refuse safely.

### State directory shape (client side)

```
state/
  hub.json
  pull/
    <profile>/
      <bundleVersion>/
        manifest.json
        bundle.tar.gz
        files/           # extracted from tarball
      latest -> <bundleVersion>   # symlink
```

Pull writes are atomic:

1. Create `state/pull/<profile>/.tmp-<random>/`.
2. Write manifest + tarball + extract files into the temp dir.
3. Verify schema, signature, checksums.
4. On success, `rename` temp dir to `state/pull/<profile>/<bundleVersion>/`.
5. Atomically replace the `latest` symlink (write `latest.new` symlink, then `rename` over `latest`).
6. On failure: `RemoveAll` the temp dir; do not touch `latest`.

### Verification order

Strict short-circuit on first failure. Each failure produces a distinct error type so callers and tests can match exact failure modes:

```go
var (
    ErrManifestUnparseable    = errors.New("manifest unparseable")
    ErrSchemaUnsupported      = errors.New("unsupported schema version")
    ErrSignatureAlgorithm     = errors.New("unsupported signature algorithm")
    ErrPinnedKeyMismatch      = errors.New("pinned public key mismatch")
    ErrSignatureInvalid       = errors.New("signature verification failed")
    ErrChecksumMismatch       = errors.New("checksum mismatch")  // already exists in bundle.LoadBundle
    ErrProfileMismatch        = errors.New("manifest profile id mismatch")
)
```

Order: parse → schema → signature.algorithm → pinnedKey → verify → bundle.LoadBundle (existing checksum verification) → profileId match.

### HTTP additions

| Route | Auth | Returns |
|---|---|---|
| `GET /api/v1/signing-key` | none | `{algorithm, publicKey}` |
| existing `/api/v1/profiles/<id>/bundle/manifest` | token (pull scope) | manifest with `signature` populated |
| existing `/api/v1/profiles/<id>/bundle` | token (pull scope) | tarball (unchanged from Slice 4) |

`/api/v1/signing-key` returning the public key unauthenticated is intentional and safe: knowing the public key only lets you verify signatures, not forge them. It also makes `init` simpler — clients can fetch and pin without consuming any token quota.

### CLI flow on init

```
confighub init --from <url> --profile <id> --token <t>
  → GET <url>/api/v1/signing-key             (no auth)
  → GET <url>/api/v1/profiles/<id>/bundle/manifest  (auth, just to validate the token works)
  → write state/hub.json {url, profile, token, pinnedPublicKey, schemaVersion: "1", lastSyncAt: null}
  → exit 0 + print "initialized; run `confighub pull` next."
```

If the auth probe fails (401/403/404), refuse to write hub.json. Init is a one-shot — the user runs it again with corrected args.

### CLI flow on pull

```
confighub pull
  → load state/hub.json
  → GET <url>/api/v1/profiles/<id>/bundle/manifest
  → check signature.algorithm == ed25519
  → verify signature against pinnedPublicKey
  → check manifest.profileId == hub.profile
  → check manifest.schemaVersion == "1"
  → GET <url>/api/v1/profiles/<id>/bundle      → bundle.tar.gz
  → extract into temp dir
  → bundle.LoadBundle(tempDir) — reuses Slice 3's checksum verification
  → on success: rename to state/pull/<profile>/<bundleVersion>/, update latest symlink, update hub.json.lastSyncAt
  → on failure: cleanup temp dir, exit non-zero, print error type
```

### CLI flow on apply --from-pulled

```
confighub apply --from-pulled
  → resolve state/pull/<profile>/latest
  → call existing internal/apply.Apply() with that bundle dir
```

Diff is the same with `diff --from-pulled`.

## Test Strategy

- `internal/sign/sign_test.go` — keypair generate, round-trip sign/verify, tamper, wrong key.
- `internal/sign/canonical_test.go` — canonical encoder is stable across runs; matches a frozen expected output for a fixture manifest.
- `internal/pull/pull_test.go` — table-driven: each rejection mode (parse error, schema mismatch, alg mismatch, key mismatch, bad signature, bad checksum, profile mismatch) produces the expected error.
- `internal/server/signing_test.go` — startup creates key file at mode 0600; subsequent startup reuses; `/api/v1/signing-key` returns only public half.
- `cmd/confighub/init_test.go` + `pull_test.go` — end-to-end against a `httptest.Server` fronting the existing handlers, asserts state/hub.json + state/pull/ shape and refuse-on-tamper.
- Smoke test against hk-cn2 after image rebuild (manual, not in `go test`).

## Risks revisited

- **Manifest schema** already has the `Signature` field nullable. Existing fixtures and the live hk-cn2 manifests serve with `signature: null`. Pull verification refuses null signatures from this slice on — so we must rebuild and redeploy the hub image before running the end-to-end test. Document in `ops/README.md`.
- **Docker volume permissions**: `state/signing-key.json` must be writable + chmod 0600 by UID 65532. The Dockerfile already ensures `/state` is owned by nonroot; the file create with `os.OpenFile(..., 0600)` should work without further setup.
- **Backwards compat for Slice 4 fixtures**: existing test fixtures under `examples/bundles/*/manifest.json` have `signature: null`. The pull verification suite will treat those as `signature unparseable / algorithm missing`. That's fine — fixtures get regenerated by a render step in the test setup, which will produce signatures once the new code is in.
