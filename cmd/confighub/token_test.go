package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenCreatePrintsOneLineAndListJSONHidesHash(t *testing.T) {
	root := t.TempDir()
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"token", "create", "--label", "macbook", "--scope", "pull:macbook", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("token create error = %v", err)
	}
	plaintext := strings.TrimSpace(out.String())
	if plaintext == "" || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("token create output = %q, want exactly one line", out.String())
	}
	files, err := filepath.Glob(filepath.Join(root, "state", "tokens", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("token files = %v err=%v, want one", files, err)
	}
	data, _ := os.ReadFile(files[0])
	if strings.Contains(string(data), plaintext) {
		t.Fatalf("plaintext persisted in token file")
	}

	cmd = newRootCommand()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"token", "list", "--root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("token list error = %v", err)
	}
	if strings.Contains(out.String(), "sha256:") || strings.Contains(out.String(), plaintext) {
		t.Fatalf("token list leaked secret material: %s", out.String())
	}
	if !strings.Contains(out.String(), `"label": "macbook"`) {
		t.Fatalf("token list missing label: %s", out.String())
	}
}
