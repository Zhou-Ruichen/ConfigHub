package pull

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/sign"
)

func TestPullSuccessAndFailures(t *testing.T) {
	base := makeFixture(t)
	cases := []struct {
		name string
		mut  func(*fixture)
		want error
	}{
		{name: "success"},
		{name: "unsupported schema", mut: func(f *fixture) { f.manifest.SchemaVersion = "9"; f.resign() }, want: ErrSchemaUnsupported},
		{name: "wrong algorithm", mut: func(f *fixture) { f.manifest.Signature = &bundle.Signature{Algorithm: "rsa", Value: "x"} }, want: ErrSignatureAlgorithm},
		{name: "pinned mismatch", mut: func(f *fixture) { other, _ := sign.Generate(); f.cfg.PinnedPublicKey = other.PublicKey }, want: ErrPinnedKeyMismatch},
		{name: "bad signature", mut: func(f *fixture) { f.manifest.ChangeSummary = "tampered" }, want: ErrSignatureInvalid},
		{name: "profile mismatch", mut: func(f *fixture) { f.manifest.ProfileID = "other"; f.resign() }, want: ErrProfileMismatch},
		{name: "bad checksum", mut: func(f *fixture) { f.files["files/config.txt"] = []byte("tampered") }, want: bundle.ErrChecksumMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := base.clone()
			if tc.mut != nil {
				tc.mut(f)
			}
			stateDir := t.TempDir()
			f.server = httptest.NewServer(f.handler())
			defer f.server.Close()
			f.cfg.URL = f.server.URL
			res, err := Pull(context.Background(), f.cfg, stateDir, PullOptions{})
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("Pull() error = %v, want %v", err, tc.want)
				}
				if _, statErr := os.Lstat(filepath.Join(stateDir, "pull", f.cfg.Profile, "latest")); statErr == nil {
					t.Fatalf("latest symlink exists after failed pull")
				}
				return
			}
			if err != nil {
				t.Fatalf("Pull() error = %v", err)
			}
			if res.BundleVersion != f.manifest.BundleVersion {
				t.Fatalf("BundleVersion = %q, want %q", res.BundleVersion, f.manifest.BundleVersion)
			}
			if _, err := os.Stat(filepath.Join(res.Path, "manifest.json")); err != nil {
				t.Fatalf("manifest not installed: %v", err)
			}
			if target, err := os.Readlink(filepath.Join(stateDir, "pull", f.cfg.Profile, "latest")); err != nil || target != f.manifest.BundleVersion {
				t.Fatalf("latest target=%q err=%v", target, err)
			}
		})
	}
}

func TestPullRejectsUnparseableManifest(t *testing.T) {
	f := makeFixture(t)
	stateDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/signing-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"algorithm": "ed25519", "publicKey": base64.StdEncoding.EncodeToString(f.kp.PublicKey)})
		case "/api/v1/profiles/macbook/bundle/manifest":
			_, _ = w.Write([]byte("{"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	f.cfg.URL = server.URL
	_, err := Pull(context.Background(), f.cfg, stateDir, PullOptions{})
	if !errors.Is(err, ErrManifestUnparseable) {
		t.Fatalf("Pull() error = %v, want ErrManifestUnparseable", err)
	}
}

type fixture struct {
	kp       *sign.Keypair
	manifest *bundle.Manifest
	files    map[string][]byte
	cfg      *HubConfig
	server   *httptest.Server
}

func makeFixture(t *testing.T) *fixture {
	t.Helper()
	kp, err := sign.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	data := []byte("hello\n")
	manifest := &bundle.Manifest{SchemaVersion: "1", BundleVersion: "v1", ProfileID: "macbook", CreatedAt: "2026-05-16T20:00:00Z", SourceRevision: "test", Domains: []string{"dotfiles"}, Files: []bundle.FileEntry{{TemplateID: "dotfiles/config", Domain: "dotfiles", BundlePath: "files/config.txt", TargetPath: "~/.config/confighub/test/config.txt", Mode: "0600", Checksum: checksum(data), Delivery: "sync", Safety: bundle.Safety{Backup: "required", Diff: "required", Symlink: "reject", Secrets: "forbidden", Merge: "replace"}}}, RemovedFiles: []bundle.RemovedFileEntry{}, ChangeSummary: "test"}
	f := &fixture{kp: kp, manifest: manifest, files: map[string][]byte{"files/config.txt": data}, cfg: &HubConfig{Profile: "macbook", Token: "tok", PinnedPublicKey: kp.PublicKey, SchemaVersion: "1"}}
	f.resign()
	return f
}

func (f *fixture) clone() *fixture {
	m := *f.manifest
	m.Domains = append([]string(nil), f.manifest.Domains...)
	m.Files = append([]bundle.FileEntry(nil), f.manifest.Files...)
	m.RemovedFiles = append([]bundle.RemovedFileEntry(nil), f.manifest.RemovedFiles...)
	if f.manifest.Signature != nil {
		s := *f.manifest.Signature
		m.Signature = &s
	}
	files := make(map[string][]byte, len(f.files))
	for k, v := range f.files {
		files[k] = append([]byte(nil), v...)
	}
	cfg := *f.cfg
	cfg.PinnedPublicKey = append([]byte(nil), f.cfg.PinnedPublicKey...)
	return &fixture{kp: f.kp, manifest: &m, files: files, cfg: &cfg}
}

func (f *fixture) resign() {
	f.manifest.Signature = nil
	bundle.ApplyDefaults(f.manifest)
	if f.manifest.Domains == nil {
		f.manifest.Domains = []string{}
	}
	if f.manifest.Files == nil {
		f.manifest.Files = []bundle.FileEntry{}
	}
	if f.manifest.RemovedFiles == nil {
		f.manifest.RemovedFiles = []bundle.RemovedFileEntry{}
	}
	sig, err := sign.SignManifest(f.manifest, f.kp.PrivateKey)
	if err != nil {
		panic(err)
	}
	f.manifest.Signature = sig
}

func (f *fixture) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/signing-key":
			_ = json.NewEncoder(w).Encode(map[string]string{"algorithm": "ed25519", "publicKey": base64.StdEncoding.EncodeToString(f.kp.PublicKey)})
		case "/api/v1/profiles/macbook/bundle/manifest":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(f.manifest)
		case "/api/v1/profiles/macbook/bundle":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			archive, err := f.archive()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			_, _ = w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func (f *fixture) archive() ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	manifestBytes, _ := json.MarshalIndent(f.manifest, "", "  ")
	manifestBytes = append(manifestBytes, '\n')
	entries := map[string][]byte{"manifest.json": manifestBytes}
	checksums := map[string]string{}
	for p, data := range f.files {
		entries[p] = data
		checksums[p] = checksum(data)
	}
	checksumsBytes, _ := json.MarshalIndent(checksums, "", "  ")
	entries["checksums.json"] = append(checksumsBytes, '\n')
	for _, name := range []string{"manifest.json", "checksums.json", "files/config.txt"} {
		data, ok := entries[name]
		if !ok {
			continue
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
