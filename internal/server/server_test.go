package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/secret"
	"github.com/ruichen/config-hub/internal/sign"
)

func TestStatusReturnsExpectedKeys(t *testing.T) {
	root := testRoot(t)
	srv, err := NewWithConfig(Config{RootDir: root, LoopbackOnly: true, Logger: log.New(io.Discard, "", 0), SessionKey: []byte("test-key")})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	for _, key := range []string{"version", "profiles", "tokens", "uptime"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("status body missing key %q: %#v", key, body)
		}
	}
}

func TestNonLoopbackRequiresToken(t *testing.T) {
	_, err := NewWithConfig(Config{RootDir: t.TempDir(), LoopbackOnly: false, Logger: log.New(io.Discard, "", 0), SessionKey: []byte("test-key")})
	if !errors.Is(err, ErrNoTokenConfigured) {
		t.Fatalf("New() error = %v, want ErrNoTokenConfigured", err)
	}
}

func TestAuthMissingValidAndWrongScope(t *testing.T) {
	root := testRoot(t)
	pullToken, _, err := secret.Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create pull token: %v", err)
	}
	wrongToken, _, err := secret.Create(root, "other", "pull:other")
	if err != nil {
		t.Fatalf("Create wrong token: %v", err)
	}
	srv := newAuthServer(t, root, nil)

	if code := requestCode(srv, "GET", "/api/v1/profiles", ""); code != http.StatusUnauthorized {
		t.Fatalf("missing token /profiles code = %d, want 401", code)
	}
	if code := requestCode(srv, "GET", "/api/v1/profiles", pullToken); code != http.StatusOK {
		t.Fatalf("valid token /profiles code = %d, want 200", code)
	}
	if code := requestCode(srv, "GET", "/api/v1/profiles/macbook", wrongToken); code != http.StatusForbidden {
		t.Fatalf("wrong scope profile code = %d, want 403", code)
	}
	if code := requestCode(srv, "GET", "/api/v1/profiles/macbook/templates/ai/claude", ""); code != http.StatusNotFound {
		t.Fatalf("unauth secret template code = %d, want 404", code)
	}
	if code := requestCode(srv, "GET", "/api/v1/profiles/macbook/templates/ai/claude", pullToken); code != http.StatusNotFound {
		t.Fatalf("wrong token secret template code = %d, want 404", code)
	}
}

func TestBundleArchiveETagAndManifest(t *testing.T) {
	root := testRoot(t)
	plaintext, _, err := secret.Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	srv := newAuthServer(t, root, nil)

	rec := authRequest(srv, "GET", "/api/v1/profiles/macbook/bundle", plaintext)
	if rec.Code != http.StatusOK {
		t.Fatalf("bundle code=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/gzip") {
		t.Fatalf("Content-Type = %q, want application/gzip", got)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("missing ETag")
	}
	assertArchiveContains(t, rec.Body.Bytes(), "manifest.json")

	req := httptest.NewRequest("GET", "/api/v1/profiles/macbook/bundle", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	req.Header.Set("If-None-Match", etag)
	rec304 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec304, req)
	if rec304.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match code = %d, want 304", rec304.Code)
	}

	manifest := authRequest(srv, "GET", "/api/v1/profiles/macbook/bundle/manifest", plaintext)
	if manifest.Code != http.StatusOK {
		t.Fatalf("manifest code=%d body=%s", manifest.Code, manifest.Body.String())
	}
	if !strings.Contains(manifest.Body.String(), `"profileId"`) {
		t.Fatalf("manifest body missing profileId: %s", manifest.Body.String())
	}
}

func TestDirectTemplateReadETagRemoteAndSecretRules(t *testing.T) {
	root := testRoot(t)
	readToken, _, err := secret.Create(root, "claude", "read:ai/claude")
	if err != nil {
		t.Fatalf("Create read token: %v", err)
	}
	adminToken, _, err := secret.Create(root, "admin", "admin")
	if err != nil {
		t.Fatalf("Create admin token: %v", err)
	}
	srv := newAuthServer(t, root, nil)

	plain := authRequest(srv, "GET", "/api/v1/profiles/macbook/templates/dotfiles/git", adminToken)
	if plain.Code != http.StatusNotFound {
		t.Fatalf("non-remote template code = %d, want 404", plain.Code)
	}
	secretResp := authRequest(srv, "GET", "/api/v1/profiles/macbook/templates/ai/claude", readToken)
	if secretResp.Code != http.StatusOK {
		t.Fatalf("secret template code = %d body=%s", secretResp.Code, secretResp.Body.String())
	}
	etag := secretResp.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("missing template ETag")
	}
	req := httptest.NewRequest("GET", "/api/v1/profiles/macbook/templates/ai/claude", nil)
	req.Header.Set("Authorization", "Bearer "+readToken)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("template If-None-Match code = %d, want 304", rec.Code)
	}
}

func TestLogsDoNotContainPlaintextToken(t *testing.T) {
	root := testRoot(t)
	plaintext, _, err := secret.Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	var logs bytes.Buffer
	srv := newAuthServer(t, root, &logs)
	_ = authRequest(srv, "GET", "/api/v1/profiles", plaintext)
	if strings.Contains(logs.String(), plaintext) {
		t.Fatalf("logs contain plaintext token: %s", logs.String())
	}
}

func TestSigningKeyLifecycleEndpointAndManifestSignature(t *testing.T) {
	root := testRoot(t)
	plaintext, _, err := secret.Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	srv := newAuthServer(t, root, nil)
	keyPath := filepath.Join(root, "state", "signing-key.json")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("signing key not created: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("signing key mode = %o, want 0600", got)
	}
	firstData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read signing key: %v", err)
	}
	var keyFile map[string]any
	if err := json.Unmarshal(firstData, &keyFile); err != nil {
		t.Fatalf("parse signing key: %v", err)
	}
	priv, _ := keyFile["privateKey"].(string)
	pub, _ := keyFile["publicKey"].(string)
	if priv == "" || pub == "" {
		t.Fatalf("signing key missing key material: %#v", keyFile)
	}

	srv2 := newAuthServer(t, root, nil)
	secondData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("reread signing key: %v", err)
	}
	if !bytes.Equal(firstData, secondData) {
		t.Fatalf("second server startup rewrote signing key")
	}

	keyResp := authRequest(srv2, "GET", "/api/v1/signing-key", "")
	if keyResp.Code != http.StatusOK {
		t.Fatalf("signing-key code=%d body=%s", keyResp.Code, keyResp.Body.String())
	}
	if strings.Contains(keyResp.Body.String(), priv) || strings.Contains(keyResp.Body.String(), "privateKey") {
		t.Fatalf("signing-key leaked private material: %s", keyResp.Body.String())
	}
	var keyBody struct {
		Algorithm string `json:"algorithm"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.Unmarshal(keyResp.Body.Bytes(), &keyBody); err != nil {
		t.Fatalf("parse signing-key response: %v", err)
	}
	if keyBody.Algorithm != "ed25519" || keyBody.PublicKey != pub {
		t.Fatalf("signing-key response = %#v, want public key %q", keyBody, pub)
	}
	publicBytes, err := base64.StdEncoding.DecodeString(keyBody.PublicKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}

	manifestResp := authRequest(srv, "GET", "/api/v1/profiles/macbook/bundle/manifest", plaintext)
	if manifestResp.Code != http.StatusOK {
		t.Fatalf("manifest code=%d body=%s", manifestResp.Code, manifestResp.Body.String())
	}
	var manifest struct {
		Signature *struct {
			Algorithm string `json:"algorithm"`
			Value     string `json:"value"`
		} `json:"signature"`
	}
	if err := json.Unmarshal(manifestResp.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.Signature == nil || manifest.Signature.Algorithm != "ed25519" || manifest.Signature.Value == "" {
		t.Fatalf("manifest missing signature: %s", manifestResp.Body.String())
	}
	parsed, err := bundle.ParseManifest(manifestResp.Body.Bytes())
	if err != nil {
		t.Fatalf("ParseManifest signed manifest: %v", err)
	}
	if err := sign.VerifyManifest(parsed, parsed.Signature, publicBytes); err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}
}

func newAuthServer(t *testing.T, root string, logs *bytes.Buffer) *Server {
	t.Helper()
	out := io.Discard
	if logs != nil {
		out = logs
	}
	srv, err := NewWithConfig(Config{RootDir: root, LoopbackOnly: false, Logger: log.New(out, "", 0), SessionKey: []byte("test-session-key")})
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}
	return srv
}

func requestCode(srv *Server, method, path, token string) int {
	return authRequest(srv, method, path, token).Code
}

func authRequest(srv *Server, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func testRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "examples", "profiles"), filepath.Join(root, "profiles"))
	copyDir(t, filepath.Join("..", "..", "examples", "templates"), filepath.Join(root, "templates"))
	copyDir(t, filepath.Join("..", "..", "examples", "bundles"), filepath.Join(root, "bundles"))
	manifestPath := filepath.Join(root, "bundles", "macbook", "2026-05-16T15-04-22Z-001", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	data = bytes.Replace(data, []byte(`"delivery": "sync"`), []byte(`"delivery": "both"`), 1)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	templatePath := filepath.Join(root, "templates", "ai", "claude.yaml")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	templateData = bytes.Replace(templateData, []byte("  remote: false"), []byte("  remote: true"), 1)
	if err := os.WriteFile(templatePath, templateData, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	return root
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o644)
	}); err != nil {
		t.Fatalf("copyDir %s -> %s: %v", src, dst, err)
	}
}

func assertArchiveContains(t *testing.T, data []byte, name string) {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		if h.Name == name {
			return
		}
	}
	t.Fatalf("archive missing %q", name)
}
