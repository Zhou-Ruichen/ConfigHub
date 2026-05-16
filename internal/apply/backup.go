package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func FileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return Checksum(data), nil
}

func CreateBackupDir(home, bundleVersion string) (string, error) {
	dir := filepath.Join(home, ".confighub", "backups", bundleVersion)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("chmod backup dir: %w", err)
	}
	return dir, nil
}

// BackupFile copies an existing target into backupDir and returns the path
// relative to backupDir. Mapping: home-relative dotfiles are stored under a
// collision-safe tree where ~/.gitconfig becomes dotfiles/gitconfig.bak, and
// nested paths keep their home-relative structure with a .bak suffix, e.g.
// ~/.config/tool/config.json -> .config/tool/config.json.bak.
func BackupFile(backupDir, resolvedTarget string) (string, error) {
	info, err := os.Lstat(resolvedTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat target for backup %q: %w", resolvedTarget, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("backup %q: directories are not supported", resolvedTarget)
	}

	home := backupHomeFromDir(backupDir)
	rel, err := filepath.Rel(home, resolvedTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = strings.TrimPrefix(filepath.VolumeName(resolvedTarget), string(filepath.Separator))
	}
	rel = backupRelPath(rel)
	backupPath := filepath.Join(backupDir, rel)
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o700); err != nil {
		return "", fmt.Errorf("create backup parent: %w", err)
	}

	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o600
	}
	in, err := os.Open(resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("open backup source: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("copy backup file: %w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("fsync backup file: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close backup file: %w", err)
	}
	if err := os.Chmod(backupPath, mode); err != nil {
		return "", fmt.Errorf("chmod backup file: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

func backupHomeFromDir(backupDir string) string {
	// backupDir is ~/.confighub/backups/<bundleVersion>; three Dir calls return ~.
	return filepath.Dir(filepath.Dir(filepath.Dir(backupDir)))
}

func backupRelPath(rel string) string {
	rel = filepath.Clean(rel)
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || rel == "." {
		rel = strings.TrimLeft(filepath.ToSlash(rel), "/")
	}
	if !strings.Contains(rel, string(filepath.Separator)) && strings.HasPrefix(rel, ".") {
		rel = filepath.Join("dotfiles", strings.TrimPrefix(rel, ".")+".bak")
	} else {
		rel += ".bak"
	}
	return rel
}
