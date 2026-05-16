package pull

import (
	"fmt"
	"os"
	"path/filepath"
)

func updateLatest(profileDir, version string) error {
	newLink := filepath.Join(profileDir, "latest.new")
	latest := filepath.Join(profileDir, "latest")
	_ = os.Remove(newLink)
	if err := os.Symlink(version, newLink); err != nil {
		return fmt.Errorf("create latest symlink: %w", err)
	}
	if err := os.Rename(newLink, latest); err != nil {
		_ = os.Remove(newLink)
		return fmt.Errorf("update latest symlink: %w", err)
	}
	if dir, err := os.Open(profileDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func LatestBundleDir(stateDir, profile string) (string, error) {
	latest := filepath.Join(stateDir, "pull", profile, "latest")
	target, err := os.Readlink(latest)
	if err != nil {
		return "", fmt.Errorf("no successful pull exists for profile %q", profile)
	}
	if filepath.IsAbs(target) {
		return target, nil
	}
	return filepath.Join(filepath.Dir(latest), target), nil
}
