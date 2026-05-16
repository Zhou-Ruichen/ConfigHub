package secret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreatePersistsHashOnlyWith0600(t *testing.T) {
	root := t.TempDir()
	plaintext, tok, err := Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(plaintext, "cfh_") {
		t.Fatalf("plaintext token has unexpected prefix: %q", plaintext)
	}
	path := filepath.Join(root, "state", "tokens", tok.ID+".json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token file mode = %o, want 0600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if strings.Contains(string(data), plaintext) {
		t.Fatalf("token plaintext was persisted")
	}
	if !strings.Contains(string(data), "sha256:") {
		t.Fatalf("token hash missing from file: %s", data)
	}
}

func TestLookupMatchesPlaintextAndRejectsWrongToken(t *testing.T) {
	root := t.TempDir()
	plaintext, tok, err := Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := Lookup(root, plaintext)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if got.ID != tok.ID {
		t.Fatalf("Lookup() id = %q, want %q", got.ID, tok.ID)
	}
	if _, err := Lookup(root, "cfh_wrong"); !errors.Is(err, ErrUnknownToken) {
		t.Fatalf("Lookup(wrong) error = %v, want ErrUnknownToken", err)
	}
}

func TestRevokeRemovesFileAndSecondRevokeFails(t *testing.T) {
	root := t.TempDir()
	_, tok, err := Create(root, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := Revoke(root, tok.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "state", "tokens", tok.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("token file still exists or stat error = %v", err)
	}
	if err := Revoke(root, tok.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("second Revoke() error = %v, want ErrTokenNotFound", err)
	}
}

func TestCreateRejectsInvalidScopes(t *testing.T) {
	root := t.TempDir()
	for _, scope := range []string{"", "pull:", "read:", "write:macbook", "admin:all"} {
		if _, _, err := Create(root, "bad", scope); !errors.Is(err, ErrInvalidScope) {
			t.Fatalf("Create(scope %q) error = %v, want ErrInvalidScope", scope, err)
		}
	}
}
