package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicSuccessCleansTmp(t *testing.T) {
	root := t.TempDir()
	manifest := testManifest("v1")
	files := map[string][]byte{
		"files/example.txt": []byte("hello\n"),
	}
	checksums := map[string]string{
		"files/example.txt": "sha256:test",
	}

	if err := WriteAtomic(root, "macbook", "v1", files, manifest, checksums); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "macbook", "v1", "files", "example.txt")); err != nil {
		t.Fatalf("rendered file not written: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, "macbook", "v1", "files", "example.txt"))
	if err != nil {
		t.Fatalf("stat rendered file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("rendered file mode = %v, want 0644", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".tmp")); !os.IsNotExist(err) {
		t.Fatalf(".tmp exists after success, err=%v", err)
	}
}

func TestWriteAtomicFailureCleansTmp(t *testing.T) {
	root := t.TempDir()
	version := "v1"
	if err := os.MkdirAll(filepath.Join(root, "macbook"), 0o755); err != nil {
		t.Fatalf("seed profile dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "macbook", version), []byte("blocks directory rename"), 0o644); err != nil {
		t.Fatalf("seed blocking final path: %v", err)
	}

	manifest := testManifest(version)
	files := map[string][]byte{
		"files/output.txt": []byte("hello\n"),
	}
	err := WriteAtomic(root, "macbook", version, files, manifest, map[string]string{})
	if err == nil {
		t.Fatalf("WriteAtomic() error = nil, want error")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".tmp")); !os.IsNotExist(statErr) {
		t.Fatalf(".tmp exists after failure, err=%v", statErr)
	}
	if info, statErr := os.Stat(filepath.Join(root, "macbook", version)); statErr != nil || info.IsDir() {
		t.Fatalf("final blocking file changed, info=%v err=%v", info, statErr)
	}
}

func testManifest(version string) *Manifest {
	return &Manifest{
		SchemaVersion:  SupportedSchemaVersion,
		BundleVersion:  version,
		ProfileID:      "macbook",
		CreatedAt:      "2026-05-16T15:04:22Z",
		SourceRevision: "none",
		Domains:        []string{"dotfiles"},
		Files: []FileEntry{
			{
				TemplateID: "dotfiles/example",
				Domain:     "dotfiles",
				BundlePath: "files/example.txt",
				TargetPath: "~/.example",
				Mode:       "0644",
				Checksum:   "sha256:test",
				Delivery:   "sync",
				Safety: Safety{
					Backup:  "required",
					Diff:    "required",
					Symlink: "reject",
					Secrets: "forbidden",
					Merge:   "replace",
				},
			},
		},
		RemovedFiles:  []RemovedFileEntry{},
		ChangeSummary: "",
		Signature:     nil,
	}
}
