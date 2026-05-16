# Slice 5 Implementation Plan

Execution order matters: signing primitives → server signing → client pull → CLI wiring → tests. Each block lists files to create or modify.

## Phase 1: Signing primitives (`internal/sign/`)

Create `internal/sign/`:

- `internal/sign/keypair.go`
  - `type Keypair struct { Algorithm string; PublicKey []byte; PrivateKey []byte; CreatedAt time.Time }`
  - `func Generate() (*Keypair, error)` — wraps `ed25519.GenerateKey(rand.Reader)`.
  - `func Load(path string) (*Keypair, error)` — reads + parses + base64-decodes the JSON file. Errors on mode > 0600.
  - `func (*Keypair) Save(path string) error` — atomic write with mode 0600. Refuses to overwrite existing files; caller decides rotate flow.
- `internal/sign/canonical.go`
  - `func CanonicalManifestBytes(m *bundle.Manifest) ([]byte, error)` — zero `Signature` to nil, marshal with `SetEscapeHTML(false)`, no trailing newline.
- `internal/sign/sign.go`
  - `func SignManifest(m *bundle.Manifest, priv ed25519.PrivateKey) (*bundle.Signature, error)`
  - `func VerifyManifest(m *bundle.Manifest, sig *bundle.Signature, pub ed25519.PublicKey) error` — returns typed errors from the table below.
- `internal/sign/errors.go`
  - `var ErrSignatureAlgorithm, ErrSignatureInvalid` (others live where they're used).
- `internal/sign/sign_test.go`
  - Round-trip, tamper byte, wrong key, missing signature field, wrong algorithm.
- `internal/sign/canonical_test.go`
  - Same manifest → same canonical bytes; manifest with `signature` populated vs nil → same canonical bytes.

## Phase 2: Server-side signing key + endpoint

Create `internal/server/signing.go`:

- `func (s *Server) ensureSigningKey(stateDir string) error` — called once during `Server.Start()`. Generates if missing, loads if present. Stores `*sign.Keypair` on the server struct.
- Augment the existing manifest-serve handler: just before writing response, call `sign.SignManifest(&manifest, server.keypair.PrivateKey)` and set the result on `manifest.Signature`.
- Add route `GET /api/v1/signing-key` returning `{algorithm, publicKey}`. Public, no auth.

Files to modify:

- `internal/server/types.go` — add `keypair *sign.Keypair` to `Server`.
- `internal/server/web.go` — register `/api/v1/signing-key` route; modify `handleBundleManifest` to call sign before writing.
- `internal/server/server_test.go` — add tests:
  - First Start() creates key file at correct mode.
  - Second Start() reuses existing key.
  - `/api/v1/signing-key` returns the right shape and no private material.
  - Bundle manifest is now signed and verifies with the served public key.

## Phase 3: Client pull package (`internal/pull/`)

Create `internal/pull/`:

- `internal/pull/hubconfig.go`
  - `type HubConfig struct { URL, Profile, Token string; PinnedPublicKey []byte; SchemaVersion string; LastSyncAt *time.Time }`
  - `func Load(stateDir string) (*HubConfig, error)`, `func (*HubConfig) Save(stateDir string) error` (atomic, mode 0600).
- `internal/pull/client.go`
  - `type Client struct { http *http.Client; cfg *HubConfig }`
  - `func New(cfg *HubConfig) *Client`
  - `func (c *Client) FetchSigningKey(ctx) ([]byte, error)`
  - `func (c *Client) FetchManifest(ctx) (*bundle.Manifest, error)`
  - `func (c *Client) FetchBundle(ctx) (io.ReadCloser, error)`
- `internal/pull/pull.go`
  - `func Pull(ctx, cfg *HubConfig, stateDir string, opts PullOptions) (*PullResult, error)` — orchestrates fetch + verify + extract + atomic install.
  - Verifies in this order: schema → algorithm → pinned key → signature → profile id → checksums (delegated to `bundle.LoadBundle` on the extracted dir).
- `internal/pull/errors.go`
  - Typed errors from design doc.
- `internal/pull/symlink.go`
  - `func updateLatest(profileDir, version string) error` — atomic via `latest.new` rename trick.
- `internal/pull/pull_test.go` — table-driven failures, httptest-backed.

## Phase 4: CLI surface (`cmd/confighub/`)

- `cmd/confighub/init.go` — new file. Cobra subcommand `init`. Args: `--from, --profile, --token`.
  - Probes `/api/v1/signing-key`, then `/api/v1/profiles/<id>/bundle/manifest` (sanity check auth).
  - Writes `state/hub.json`.
- `cmd/confighub/pull.go` — new file. Cobra subcommand `pull` with `--dry-run`.
  - Calls `pull.Pull`.
  - On `--dry-run`: do everything except install (no atomic rename, no symlink update). Print "would write to <path>".
- `cmd/confighub/apply.go` — modify. Add `--from-pulled` flag that resolves `state/pull/<profile>/latest` and feeds the existing apply path.
- `cmd/confighub/diff.go` — if a diff command exists, add `--from-pulled`. If diff is currently a subcommand of apply, expose it as needed. (Check first; the codebase may have `confighub diff` already.)
- `cmd/confighub/main.go` — register the new commands.

## Phase 5: Tests + fixtures

- Add a `httptest.Server`-based integration test in `cmd/confighub/cli_pull_test.go` covering the full init → pull → apply happy path against a fixture profile.
- Add negative-path tests for each rejection mode under `internal/pull/pull_test.go` (mock bundle data, tamper, etc.).
- Update `ops/README.md` with the rebuild-and-redeploy note (existing hk-cn2 manifests were rendered without signatures and need a fresh render after upgrade).

## Phase 6: Verification before reporting done

Pi must run before reporting:

```bash
go vet ./...
go test ./...
go build ./cmd/confighub
```

Then smoke test locally:

```bash
TMPDIR=$(mktemp -d)
mkdir -p $TMPDIR/hub-state $TMPDIR/client-state
HOME=$TMPDIR/client-home

# Start a hub on a random port (test fixture profile already in examples/profiles/)
./confighub serve --bind 127.0.0.1:0 --state $TMPDIR/hub-state --profiles examples/profiles --templates examples/templates &
HUB_PID=$!
# ...read assigned port from stdout, capture as $PORT...
TOKEN=$(./confighub token create --state $TMPDIR/hub-state --scope pull --profile macbook)

./confighub init --from http://127.0.0.1:$PORT --profile macbook --token $TOKEN --state $TMPDIR/client-state
./confighub pull --state $TMPDIR/client-state
./confighub apply --from-pulled --state $TMPDIR/client-state --dry-run

kill $HUB_PID
```

If `confighub serve --bind 127.0.0.1:0` isn't already supported, dispatch with a fixed port and document the test setup.

## Non-Goals (push to later slices)

- Key rotation flow (`--accept-new-key`)
- Delta pull / incremental sync
- TLS termination
- Audit log for pull events
- Hub-side rate limiting

## Open Questions for Orchestrator

None — all design decisions are made in `design.md`. If Pi hits a real ambiguity, it should report it as a blocker, not invent a behavior.
