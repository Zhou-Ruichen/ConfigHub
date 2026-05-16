package apply

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruichen/config-hub/internal/bundle"
)

func TestApplyRoundTripNoopRollbackAndLogRedaction(t *testing.T) {
	root, home, bundleDir := writeApplyFixture(t)
	writeFileMode(t, filepath.Join(home, ".gitconfig"), []byte("[user]\n\temail = before@example.com\n"), 0o644)
	writeFileMode(t, filepath.Join(home, ".claude", "settings.json"), []byte("old-json-secret\n"), 0o600)

	res, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if res.BackupDir == "" || len(res.Files) != 3 {
		t.Fatalf("Apply result = %#v", res)
	}
	assertFile(t, filepath.Join(home, ".claude", "settings.json"), "{\n  \"managed\": \"confighub\"\n}\n")
	assertFile(t, filepath.Join(home, ".config", "confighub", "fragments", "dotfiles", "git", "confighub.gitconfig"), "[user]\n\tname = ruichen\n")
	gitconfig := readFile(t, filepath.Join(home, ".gitconfig"))
	if !strings.Contains(gitconfig, "[user]\n\temail = before@example.com") || !strings.Contains(gitconfig, "# >>> confighub:dotfiles-git-include >>>") {
		t.Fatalf("gitconfig content = %q", gitconfig)
	}

	second, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true})
	if err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	for _, action := range second.Files {
		if action.Action != ActionUnchanged {
			t.Fatalf("second action = %#v, want unchanged", action)
		}
	}
	if entries, _ := os.ReadDir(second.BackupDir); len(entries) != 0 {
		t.Fatalf("noop backup dir has entries: %#v", entries)
	}

	if _, err := Rollback(RollbackOptions{RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true}); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	assertFile(t, filepath.Join(home, ".gitconfig"), "[user]\n\temail = before@example.com\n")
	assertFile(t, filepath.Join(home, ".claude", "settings.json"), "old-json-secret\n")
	if _, err := os.Stat(filepath.Join(home, ".config", "confighub", "fragments", "dotfiles", "git", "confighub.gitconfig")); !os.IsNotExist(err) {
		t.Fatalf("new fragment after rollback stat err = %v, want not exist", err)
	}

	logData := readFile(t, filepath.Join(root, "state", "apply.log"))
	for _, line := range strings.Split(strings.TrimSpace(logData), "\n") {
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("apply log is not JSON: %v", err)
		}
	}
	if strings.Contains(logData, "old-json-secret") || strings.Contains(logData, "rendered-secret") {
		t.Fatalf("apply log contains secret-like content: %s", logData)
	}
}

func TestManagedSectionAppendOncePreservesSurroundingContent(t *testing.T) {
	root, home, bundleDir := writeApplyFixture(t)
	target := filepath.Join(home, ".gitconfig")
	writeFileMode(t, target, []byte("top\n# user block\nbottom\n"), 0o644)
	if _, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	first := readFile(t, target)
	if !strings.Contains(first, "top\n# user block\nbottom") || !strings.Contains(first, "# >>> confighub:dotfiles-git-include >>>") {
		t.Fatalf("first gitconfig = %q", first)
	}
	if _, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true}); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if got := readFile(t, target); got != first {
		t.Fatalf("append-once not idempotent\nfirst=%q\ngot=%q", first, got)
	}
	if _, err := Rollback(RollbackOptions{RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true}); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	assertFile(t, target, "top\n# user block\nbottom\n")
}

func TestPathPolicyRejectsForbidden(t *testing.T) {
	home := t.TempDir()
	for _, target := range []string{"~/.ssh/config", "~/.gnupg/pubring.kbx", "~/.config/app/history", "~/.config/app/state.db", "~/.cache"} {
		if _, err := ValidateTargetPath(home, target); !errors.Is(err, ErrPathPolicy) {
			t.Fatalf("ValidateTargetPath(%q) err = %v, want ErrPathPolicy", target, err)
		}
	}
}

func TestChecksumMismatch(t *testing.T) {
	_, _, bundleDir := writeApplyFixture(t)
	path := filepath.Join(bundleDir, "files", "dotfiles", "git", "confighub.gitconfig")
	if err := os.WriteFile(path, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := bundle.LoadBundle(bundleDir)
	if !errors.Is(err, bundle.ErrChecksumMismatch) {
		t.Fatalf("LoadBundle() err = %v, want ErrChecksumMismatch", err)
	}
}

func TestSymlinkRejectAndReplace(t *testing.T) {
	root, home, bundleDir := writeApplyFixture(t)
	target := filepath.Join(home, ".config", "confighub", "fragments", "dotfiles", "git", "confighub.gitconfig")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDest := filepath.Join(home, ".config", "other")
	writeFileMode(t, linkDest, []byte("other"), 0o644)
	if err := os.Symlink(linkDest, target); err != nil {
		t.Fatal(err)
	}
	_, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true})
	if !errors.Is(err, ErrPathPolicy) {
		t.Fatalf("Apply symlink err = %v, want ErrPathPolicy", err)
	}

	manifest := loadManifestForEdit(t, bundleDir)
	for i := range manifest.Files {
		if manifest.Files[i].TemplateID == "dotfiles/git" {
			manifest.Files[i].Safety.Symlink = "replace"
		}
	}
	writeManifest(t, bundleDir, manifest)
	_ = os.Remove(filepath.Join(root, "state", "apply.lock"))
	if _, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true}); err != nil {
		t.Fatalf("Apply replace symlink error = %v", err)
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("target not regular after replace info=%v err=%v", info, err)
	}
}

func TestRemovedFiles(t *testing.T) {
	root, home, bundleDir := writeApplyFixture(t)
	removed := filepath.Join(home, ".config", "old", "gone.conf")
	writeFileMode(t, removed, []byte("old\n"), 0o644)
	manifest := loadManifestForEdit(t, bundleDir)
	manifest.RemovedFiles = []bundle.RemovedFileEntry{{TemplateID: "old/template", TargetPath: "~/.config/old/gone.conf", Reason: "removed", PreviousChecksum: Checksum([]byte("old\n"))}}
	writeManifest(t, bundleDir, manifest)
	if _, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true}); err != nil {
		t.Fatalf("Apply removed error = %v", err)
	}
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatalf("removed target stat err = %v, want not exist", err)
	}

	root, home, bundleDir = writeApplyFixture(t)
	manifest = loadManifestForEdit(t, bundleDir)
	manifest.RemovedFiles = []bundle.RemovedFileEntry{{TemplateID: "old/template", TargetPath: "~/.config/old/missing.conf", Reason: "removed", PreviousChecksum: Checksum([]byte("old\n"))}}
	writeManifest(t, bundleDir, manifest)
	res, err := Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true})
	if err != nil {
		t.Fatalf("Apply missing removed error = %v", err)
	}
	if len(res.RemovedFiles) != 1 || res.RemovedFiles[0].Action != ActionRemovedNoop {
		t.Fatalf("removed missing result = %#v", res.RemovedFiles)
	}

	root, home, bundleDir = writeApplyFixture(t)
	removed = filepath.Join(home, ".config", "old", "gone.conf")
	writeFileMode(t, removed, []byte("changed\n"), 0o644)
	manifest = loadManifestForEdit(t, bundleDir)
	manifest.RemovedFiles = []bundle.RemovedFileEntry{{TemplateID: "old/template", TargetPath: "~/.config/old/gone.conf", Reason: "removed", PreviousChecksum: Checksum([]byte("old\n"))}}
	writeManifest(t, bundleDir, manifest)
	_, err = Apply(context.Background(), Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: "macbook", Yes: true})
	if !errors.Is(err, ErrPathPolicy) {
		t.Fatalf("removed checksum mismatch err = %v, want ErrPathPolicy", err)
	}
	assertFile(t, removed, "changed\n")
}

func TestLockContention(t *testing.T) {
	state := t.TempDir()
	release, err := AcquireApplyLock(state, "macbook")
	if err != nil {
		t.Fatalf("AcquireApplyLock() error = %v", err)
	}
	defer release()
	_, err = AcquireApplyLock(state, "macbook")
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("second lock err = %v, want ErrLockHeld", err)
	}
}

func writeApplyFixture(t *testing.T) (root, home, bundleDir string) {
	t.Helper()
	root = t.TempDir()
	home = filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := &bundle.Manifest{SchemaVersion: "1", BundleVersion: "2026-05-16T16-42-10Z-001", ProfileID: "macbook", CreatedAt: "2026-05-16T16:42:10Z", SourceRevision: "none", Domains: []string{"ai", "dotfiles"}, Files: []bundle.FileEntry{
		{TemplateID: "ai/claude", Domain: "ai", BundlePath: "files/ai/claude/settings.json", TargetPath: "~/.claude/settings.json", Mode: "0600", Checksum: Checksum([]byte("{\n  \"managed\": \"confighub\"\n}\n")), Delivery: "sync", Safety: bundle.Safety{Backup: "required", Diff: "required", Symlink: "reject", Secrets: "allowed", Merge: "replace"}},
		{TemplateID: "dotfiles/git", Domain: "dotfiles", BundlePath: "files/dotfiles/git/confighub.gitconfig", TargetPath: "~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig", Mode: "0644", Checksum: Checksum([]byte("[user]\n\tname = ruichen\n")), Delivery: "sync", Safety: bundle.Safety{Backup: "required", Diff: "required", Symlink: "reject", Secrets: "forbidden", Merge: "replace"}},
		{TemplateID: "dotfiles/git-include", Domain: "dotfiles", BundlePath: "files/dotfiles/git/include-line.txt", TargetPath: "~/.gitconfig", Mode: "0644", Checksum: Checksum([]byte("[include]\n\tpath = ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig\n")), Delivery: "sync", Safety: bundle.Safety{Backup: "required", Diff: "required", Symlink: "reject", Secrets: "forbidden", Merge: "managed-section", IncludeStrategy: "append-once"}},
	}, RemovedFiles: []bundle.RemovedFileEntry{}, Signature: nil}
	files := map[string][]byte{
		"files/ai/claude/settings.json":          []byte("{\n  \"managed\": \"confighub\"\n}\n"),
		"files/dotfiles/git/confighub.gitconfig": []byte("[user]\n\tname = ruichen\n"),
		"files/dotfiles/git/include-line.txt":    []byte("[include]\n\tpath = ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig\n"),
	}
	checksums := map[string]string{}
	for path, data := range files {
		checksums[path] = Checksum(data)
	}
	if err := bundle.WriteAtomic(filepath.Join(root, "bundles"), "macbook", manifest.BundleVersion, files, manifest, checksums); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "state", "active-profile"), []byte("macbook\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, home, filepath.Join(root, "bundles", "macbook", manifest.BundleVersion)
}

func loadManifestForEdit(t *testing.T, bundleDir string) *bundle.Manifest {
	t.Helper()
	m, err := bundle.LoadManifest(filepath.Join(bundleDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func writeManifest(t *testing.T, bundleDir string, manifest *bundle.Manifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFileMode(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
