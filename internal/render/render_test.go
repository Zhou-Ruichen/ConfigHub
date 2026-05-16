package render

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ruichen/config-hub/internal/bundle"
)

func TestRenderExampleProfile(t *testing.T) {
	root := writeRenderFixture(t)
	result, err := Render(context.Background(), Options{
		ProfilePath: root + "/profiles/macbook.yaml",
		RootDir:     root,
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	manifest := result.Manifest
	if manifest.SchemaVersion != bundle.SupportedSchemaVersion {
		t.Fatalf("SchemaVersion = %q", manifest.SchemaVersion)
	}
	if manifest.ProfileID != "macbook" || manifest.Signature != nil || manifest.SourceRevision != "none" {
		t.Fatalf("manifest header = %#v", manifest)
	}
	if !reflect.DeepEqual(manifest.Domains, []string{"ai", "dotfiles"}) {
		t.Fatalf("Domains = %#v", manifest.Domains)
	}
	if len(manifest.Files) != 3 {
		t.Fatalf("len(Files) = %d, want 3", len(manifest.Files))
	}
	if len(manifest.RemovedFiles) != 0 {
		t.Fatalf("RemovedFiles = %#v, want empty", manifest.RemovedFiles)
	}

	wantPaths := map[string]bool{
		"files/ai/claude/settings.json":          true,
		"files/dotfiles/git/confighub.gitconfig": true,
		"files/dotfiles/git/include-line.txt":    true,
	}
	for _, entry := range manifest.Files {
		if !wantPaths[entry.BundlePath] {
			t.Fatalf("unexpected bundlePath %q", entry.BundlePath)
		}
		if entry.Checksum == "" || entry.Checksum != result.Checksums[entry.BundlePath] {
			t.Fatalf("checksum mismatch for %q", entry.BundlePath)
		}
		if _, ok := result.Files[entry.BundlePath]; !ok {
			t.Fatalf("missing rendered bytes for %q", entry.BundlePath)
		}
	}
	if got := string(result.Files["files/dotfiles/git/confighub.gitconfig"]); got != "[user]\n\tname = ruichen\n" {
		t.Fatalf("gitconfig render = %q", got)
	}
}

func TestRenderRemovedFiles(t *testing.T) {
	root := writeRenderFixture(t)
	first, err := Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err != nil {
		t.Fatalf("first Render() error = %v", err)
	}
	if err := bundle.WriteAtomic(root+"/bundles", first.Manifest.ProfileID, first.Manifest.BundleVersion, first.Files, first.Manifest, first.Checksums); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	writeFile(t, root+"/profiles/macbook.yaml", `id: macbook
owner: ruichen
role: workstation
os: macos
domains:
  ai: true
  dotfiles: true
allowedTemplates:
  - ai/claude
  - dotfiles/git-include
vars:
  example: rendered-by-confighub
`)

	second, err := Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err != nil {
		t.Fatalf("second Render() error = %v", err)
	}
	if len(second.Manifest.RemovedFiles) != 1 {
		t.Fatalf("RemovedFiles = %#v, want 1", second.Manifest.RemovedFiles)
	}
	removed := second.Manifest.RemovedFiles[0]
	if removed.TemplateID != "dotfiles/git" || removed.TargetPath != "~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig" || removed.Reason != "removed" {
		t.Fatalf("RemovedFiles[0] = %#v", removed)
	}
	if removed.PreviousChecksum == "" {
		t.Fatalf("PreviousChecksum is empty")
	}
}

func TestRenderSecretErrors(t *testing.T) {
	root := writeRenderFixture(t)
	writeFile(t, root+"/templates/ai/claude/settings.json.tmpl", `{{ secret "openai_key" }}
`)
	_, err := Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err == nil {
		t.Fatalf("Render() error = nil, want secret error")
	}
}

func TestRenderEnvAllowlist(t *testing.T) {
	t.Setenv("ALLOWED_VAR", "ok")
	t.Setenv("OTHER", "not-ok")
	root := writeRenderFixture(t)
	writeFile(t, root+"/templates/ai/claude.yaml", `id: ai/claude
domain: ai
source: ai/claude/settings.json.tmpl
target: ~/.claude/settings.json
mode: "0600"
envAllowlist:
  - ALLOWED_VAR
delivery:
  sync: true
  remote: false
safety:
  backup: required
  diff: required
  symlink: reject
  secrets: allowed
  merge: managed-section
`)
	writeFile(t, root+"/templates/ai/claude/settings.json.tmpl", `{{ env "ALLOWED_VAR" }}
`)
	result, err := Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err != nil {
		t.Fatalf("Render() allowed env error = %v", err)
	}
	if got := string(result.Files["files/ai/claude/settings.json"]); got != "ok\n" {
		t.Fatalf("allowed env render = %q", got)
	}

	writeFile(t, root+"/templates/ai/claude/settings.json.tmpl", `{{ env "OTHER" }}
`)
	_, err = Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err == nil {
		t.Fatalf("Render() disallowed env error = nil, want error")
	}
}

func TestRenderChecksumsDeterministic(t *testing.T) {
	root := writeRenderFixture(t)
	first, err := Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err != nil {
		t.Fatalf("first Render() error = %v", err)
	}
	second, err := Render(context.Background(), Options{ProfilePath: root + "/profiles/macbook.yaml", RootDir: root})
	if err != nil {
		t.Fatalf("second Render() error = %v", err)
	}
	if !reflect.DeepEqual(first.Checksums, second.Checksums) {
		t.Fatalf("checksums differ: %#v != %#v", first.Checksums, second.Checksums)
	}
}

func writeRenderFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root+"/profiles/macbook.yaml", `id: macbook
owner: ruichen
role: workstation
os: macos
domains:
  ai: true
  dotfiles: true
allowedTemplates:
  - ai/claude
  - dotfiles/git
  - dotfiles/git-include
vars:
  example: rendered-by-confighub
`)
	writeFile(t, root+"/templates/ai/claude.yaml", `id: ai/claude
domain: ai
source: ai/claude/settings.json.tmpl
target: ~/.claude/settings.json
mode: "0600"
delivery:
  sync: true
  remote: false
safety:
  backup: required
  diff: required
  symlink: reject
  secrets: allowed
  merge: managed-section
`)
	writeFile(t, root+"/templates/dotfiles/git.yaml", `id: dotfiles/git
domain: dotfiles
source: dotfiles/git/confighub.gitconfig.tmpl
target: ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig
mode: "0644"
delivery:
  sync: true
  remote: false
safety:
  backup: required
  diff: required
  symlink: reject
  secrets: forbidden
  merge: replace
`)
	writeFile(t, root+"/templates/dotfiles/git-include.yaml", `id: dotfiles/git-include
domain: dotfiles
source: dotfiles/git/gitconfig-include.tmpl
target: ~/.gitconfig
mode: "0644"
delivery:
  sync: true
  remote: false
safety:
  backup: required
  diff: required
  symlink: reject
  secrets: forbidden
  merge: managed-section
  includeStrategy: append-once
`)
	writeFile(t, root+"/templates/ai/claude/settings.json.tmpl", "{\n  \"managed\": \"confighub\",\n  \"operator\": \"{{.Profile.Owner}}\"\n}\n")
	writeFile(t, root+"/templates/dotfiles/git/confighub.gitconfig.tmpl", "[user]\n\tname = {{.Profile.Owner}}\n")
	writeFile(t, root+"/templates/dotfiles/git/gitconfig-include.tmpl", "[include]\n\tpath = ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig\n")
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
