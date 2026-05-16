# Pull flow + bundle signatures (Slice 5)

## Goal

Close the round-trip: a client machine can `init` against the deployed hub, `pull` a signed bundle for its profile, `diff` it locally, and `apply` it through the existing Slice 3 apply engine. From this slice onward, every bundle that crosses the hub→client boundary must be signed with ed25519 and verified before any disk write.

End-to-end test fixture: profile `air` already lives on `hk-cn2` (`100.100.1.2:8787`). After this slice ships, running `confighub init --from http://100.100.1.2:8787 --profile air --token <pull-token>` followed by `pull` + `diff` + `apply` must produce `~/.gitconfig.local`, `~/.zshrc.local`, `~/.zprofile.local` on the local Mac with content matching the rendered bundle byte-for-byte.

## Requirements

### CLI surface

- `confighub init --from <url> --profile <id> --token <token>` — writes `state/hub.json` with `{url, profile, token, pinnedPublicKey, schemaVersion, lastSyncAt: null}`. Pins the hub's public key returned by `GET /api/v1/signing-key` at this moment; subsequent pulls reject a key change without an explicit `--accept-new-key` flag.
- `confighub pull` — fetches `GET /api/v1/profiles/<id>/bundle/manifest` then `GET /api/v1/profiles/<id>/bundle`, verifies signature against pinned key, verifies file checksums, writes the bundle (manifest + tarball + extracted files) under `state/pull/<profile>/<bundleVersion>/`, updates `state/hub.json.lastSyncAt`.
- `confighub pull --dry-run` — does the fetch + verify but does not move anything into the apply-ready slot; prints what would change.
- `confighub diff --from-pulled` — reads the most recent successfully-pulled bundle and runs the existing Slice 3 diff against the live filesystem.
- `confighub apply --from-pulled` — reads the most recent successfully-pulled bundle and runs the existing Slice 3 apply (atomic write, backup, log). Refuses if no successful pull exists.

### Server surface

- `GET /api/v1/signing-key` — returns `{algorithm: "ed25519", publicKey: "<base64>"}`. Public endpoint, no token required (only the pubkey is exposed; the private key never leaves the server).
- Existing `GET /api/v1/profiles/<id>/bundle/manifest` adds a `signature: {algorithm: "ed25519", value: "<base64>"}` field. The signed payload is the manifest JSON with `signature: null`, canonicalized (sorted keys, no extra whitespace), UTF-8 encoded.
- The render pipeline writes the signature into the manifest as the last step before serving.

### Signing key lifecycle

- On first `confighub serve` startup, if `state/signing-key.json` does not exist, generate an ed25519 keypair, write to `state/signing-key.json` (mode 0600) containing `{privateKey, publicKey, createdAt, algorithm: "ed25519"}`. Both keys are base64-encoded.
- On subsequent startups, load the existing keypair. Never overwrite without explicit operator action.
- The Docker image's volume mount (`/state`) already preserves this across container restarts on hk-cn2.

### Verification rules (client side)

A pulled bundle is **rejected and not written** if any of these fail, in this order:
1. Manifest JSON cannot be parsed.
2. `schemaVersion` is unsupported (currently `1`).
3. `signature.algorithm` is not `ed25519`.
4. Signature does not verify against the pinned public key.
5. Any file checksum in the manifest does not match the file content in the tarball.
6. Manifest's `profileId` does not match the requested profile.

Each failure mode prints a distinct, machine-greppable error and exits non-zero.

### State directory additions

```
state/
  hub.json                        # written by init; rewritten on pull (lastSyncAt) and rotate-key flow
  pull/
    <profile>/
      <bundleVersion>/
        manifest.json             # verified manifest
        bundle.tar.gz             # raw archive
        files/                    # extracted, for diff/apply
      latest -> <bundleVersion>   # symlink to the most recent verified bundle
  signing-key.json                # server only, mode 0600
```

The `state/pull/<profile>/latest` symlink lets `diff --from-pulled` and `apply --from-pulled` resolve without scanning. Pull writes are atomic: download to a temp dir, verify, then rename + update symlink.

## Acceptance Criteria

Functional:

- [ ] On a clean hk-cn2-style fixture hub, server startup creates `state/signing-key.json` with mode 0600 and a valid base64 ed25519 keypair.
- [ ] `GET /api/v1/signing-key` returns the public half only and never the private half.
- [ ] Manifests served by `/api/v1/profiles/<id>/bundle/manifest` contain a `signature` field that verifies against the public key.
- [ ] `confighub init` against the running hub writes `state/hub.json` with all five fields populated and pins the public key.
- [ ] `confighub pull` against the same hub fetches, verifies, and writes `state/pull/<profile>/<bundleVersion>/` plus the `latest` symlink.
- [ ] `confighub diff --from-pulled` shows the same diff that `confighub apply` would produce.
- [ ] `confighub apply --from-pulled` materializes the files exactly as the locally-rendered Slice 3 path would.

Negative paths:

- [ ] Tampering with `manifest.json` (any byte) after sign → pull rejects with `signature verification failed`.
- [ ] Tampering with a file inside the tarball → pull rejects with `checksum mismatch on <path>`.
- [ ] Sending a manifest signed with a different key (rotated server, client still pinned to old) → pull rejects with `pinned public key mismatch`.
- [ ] Request without token (where token is required) → `401`.
- [ ] Request for a profile the token cannot access → `404` (not `403`, to avoid leaking existence).
- [ ] Unsupported `schemaVersion` → pull rejects with `unsupported schema version`.

End-to-end against deployed hub (`hk-cn2`, `air` profile):

- [ ] `confighub init --from http://100.100.1.2:8787 --profile air --token <pull-token>` succeeds on the local Mac.
- [ ] `confighub pull` writes the air bundle locally.
- [ ] `confighub diff --from-pulled` shows three pending file creations: `~/.gitconfig.local`, `~/.zshrc.local`, `~/.zprofile.local`.
- [ ] `confighub apply --from-pulled` creates those three files.
- [ ] The `~/.gitconfig.local` contents include `name = RuichenZhou` and `email = ruichenzhou@outlook.com` exactly as rendered.
- [ ] Rerunning `apply --from-pulled` is a no-op (or a rollback-aware idempotent write).

## Out of Scope

- Key rotation flow beyond detection. (Detect + refuse + print remediation; actual rotate-and-republish wait for hardening.)
- HTTPS/TLS. Tailscale already encrypts and the hub is private to the operator.
- Hub-side rate limiting, audit log shipping.
- Delta pulls. Always pull the full latest manifest + tarball.
- AI domain templates. Slice 6.

## Test Plan

1. **Unit**: signer round-trip (sign/verify/tamper) under `internal/bundle/sign_test.go`.
2. **Unit**: pull verification rejects each failure mode listed above.
3. **Integration (local fixture hub)**: spin up `confighub serve` on `127.0.0.1:<random-port>` with a temp `state/`, run `init/pull/diff/apply` against it from a second temp `HOME`, assert files match.
4. **Smoke against hk-cn2**: run the end-to-end acceptance bullet from local Mac against the actual deployed container after rebuilding the hub image with the new signing code.

## Risks

- Manifest canonicalization mismatch between Go encoder and verifier would cause every signature to fail. Pin the encoder (sorted keys, no HTML escaping, no trailing newline) and test it.
- Existing manifests on hk-cn2 were rendered without signatures; first pull after upgrade will need a fresh render. Document this in the deploy notes (rebuild image → rerender bundles → first pull).
- The Docker image runs as UID 65532; `state/signing-key.json` must be created by that UID, not by root. Verify mode 0600 still works under nonroot.
