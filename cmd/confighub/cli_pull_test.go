package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/render"
	"github.com/ruichen/config-hub/internal/secret"
	"github.com/ruichen/config-hub/internal/server"
)

func TestCLIPullInitDiffApplyFromPulled(t *testing.T) {
	hubRoot := testCLIRoot(t)
	rendered, err := render.Render(context.Background(), render.Options{ProfilePath: filepath.Join(hubRoot, "profiles", "macbook.yaml"), RootDir: hubRoot})
	if err != nil {
		t.Fatalf("render fixture: %v", err)
	}
	if err := bundle.WriteAtomic(filepath.Join(hubRoot, "bundles"), rendered.Manifest.ProfileID, rendered.Manifest.BundleVersion, rendered.Files, rendered.Manifest, rendered.Checksums); err != nil {
		t.Fatalf("write rendered fixture: %v", err)
	}
	token, _, err := secret.Create(hubRoot, "macbook", "pull:macbook")
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	srv, err := server.NewWithConfig(server.Config{RootDir: hubRoot, LoopbackOnly: false, Logger: log.New(io.Discard, "", 0), SessionKey: []byte("test-session-key")})
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	clientRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	runCLI(t, "init", "--from", httpSrv.URL, "--profile", "macbook", "--token", token, "--root", clientRoot)
	if data, err := os.ReadFile(filepath.Join(clientRoot, "state", "hub.json")); err != nil || !bytes.Contains(data, []byte("pinnedPublicKey")) {
		t.Fatalf("hub.json missing pinnedPublicKey err=%v data=%s", err, data)
	}
	runCLI(t, "pull", "--root", clientRoot)
	latest := filepath.Join(clientRoot, "state", "pull", "macbook", "latest")
	if target, err := os.Readlink(latest); err != nil || target == "" {
		t.Fatalf("latest symlink target=%q err=%v", target, err)
	}

	diffOut := runCLI(t, "diff", "--from-pulled", "--root", clientRoot)
	if !strings.Contains(diffOut, ".gitconfig") {
		t.Fatalf("diff output missing expected target: %s", diffOut)
	}
	runCLI(t, "apply", "--from-pulled", "--root", clientRoot, "--yes")
	want := rendered.Files["files/dotfiles/git/confighub.gitconfig"]
	got, err := os.ReadFile(filepath.Join(home, ".config", "confighub", "fragments", "dotfiles", "git", "confighub.gitconfig"))
	if err != nil {
		t.Fatalf("read applied file: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("applied file mismatch\ngot: %q\nwant:%q", got, want)
	}
}

func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("confighub %v error=%v stderr=%s stdout=%s", args, err, errOut.String(), out.String())
	}
	return out.String()
}

func testCLIRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples", "profiles"), filepath.Join(root, "profiles"))
	copyTree(t, filepath.Join("..", "..", "templates"), filepath.Join(root, "templates"))
	return root
}

func copyTree(t *testing.T, src, dst string) {
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
		t.Fatalf("copyTree %s -> %s: %v", src, dst, err)
	}
}
