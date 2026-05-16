package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

func AtomicWrite(resolvedTarget string, content []byte, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o600
	}
	parent := filepath.Dir(resolvedTarget)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	tmp, err := os.CreateTemp(parent, ".confighub-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, resolvedTarget); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	cleanup = false
	if err := fsyncDir(parent); err != nil {
		return err
	}
	return nil
}

func WriteTarget(homeDir, resolvedTarget, symlinkPolicy string, content []byte, mode os.FileMode) (string, error) {
	writePath := resolvedTarget
	info, err := os.Lstat(resolvedTarget)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		switch symlinkPolicy {
		case "", "reject":
			return "", fmt.Errorf("%w: target %q is a symlink", ErrPathPolicy, resolvedTarget)
		case "replace":
			if err := os.Remove(resolvedTarget); err != nil {
				return "", fmt.Errorf("remove symlink target: %w", err)
			}
		case "follow":
			dest, err := filepath.EvalSymlinks(resolvedTarget)
			if err != nil {
				return "", fmt.Errorf("resolve symlink target: %w", err)
			}
			validated, err := ValidateTargetPath(homeDir, dest)
			if err != nil {
				return "", err
			}
			writePath = validated
		default:
			return "", fmt.Errorf("%w: unsupported symlink policy %q", ErrPathPolicy, symlinkPolicy)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat target: %w", err)
	}
	if err := AtomicWrite(writePath, content, mode); err != nil {
		return "", err
	}
	return writePath, nil
}

func fsyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open parent dir for fsync: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync parent dir: %w", err)
	}
	return nil
}
