package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// LatestBundleVersion returns the lexicographically last existing bundle version
// for profileID, or an empty string when the profile has no bundles yet.
func LatestBundleVersion(rootBundlesDir, profileID string) (string, error) {
	profileDir := filepath.Join(rootBundlesDir, profileID)
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read bundle versions for profile %q: %w", profileID, err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	if len(versions) == 0 {
		return "", nil
	}
	sort.Strings(versions)
	return versions[len(versions)-1], nil
}

// WriteAtomic writes a complete bundle tree into a temporary directory, then
// atomically promotes it to bundles/<profileID>/<version> with os.Rename.
func WriteAtomic(rootBundlesDir, profileID, version string, files map[string][]byte, manifest *Manifest, checksums map[string]string) error {
	tmpRoot := filepath.Join(rootBundlesDir, ".tmp")
	if err := os.MkdirAll(tmpRoot, 0o700); err != nil {
		return fmt.Errorf("create bundle tmp root: %w", err)
	}
	if err := os.Chmod(tmpRoot, 0o700); err != nil {
		return fmt.Errorf("chmod bundle tmp root: %w", err)
	}

	tmpDir := filepath.Join(tmpRoot, version)
	finalDir := filepath.Join(rootBundlesDir, profileID, version)
	if err := os.RemoveAll(tmpDir); err != nil {
		return fmt.Errorf("clear existing tmp bundle %q: %w", tmpDir, err)
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("create tmp bundle %q: %w", tmpDir, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
		_ = os.Remove(tmpRoot)
	}()

	fileModes := make(map[string]os.FileMode, len(manifest.Files))
	for _, entry := range manifest.Files {
		mode, err := parseFileMode(entry.Mode)
		if err != nil {
			return err
		}
		fileModes[entry.BundlePath] = mode
	}

	for bundlePath, data := range files {
		if err := validateBundlePath(bundlePath); err != nil {
			return err
		}
		path := filepath.Join(tmpDir, filepath.FromSlash(bundlePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create rendered file directory %q: %w", filepath.Dir(path), err)
		}
		mode := fileModes[bundlePath]
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			return fmt.Errorf("write rendered file %q: %w", path, err)
		}
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), manifestData, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	checksumsData, err := json.MarshalIndent(checksums, "", "  ")
	if err != nil {
		return fmt.Errorf("encode checksums: %w", err)
	}
	checksumsData = append(checksumsData, '\n')
	if err := os.WriteFile(filepath.Join(tmpDir, "checksums.json"), checksumsData, 0o644); err != nil {
		return fmt.Errorf("write checksums: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(rootBundlesDir, profileID), 0o755); err != nil {
		return fmt.Errorf("create profile bundle directory: %w", err)
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return fmt.Errorf("promote bundle %q to %q: %w", tmpDir, finalDir, err)
	}
	cleanup = false
	return nil
}

func validateBundlePath(path string) error {
	if path == "" {
		return fmt.Errorf("validate bundle path: empty path")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(path) || clean == "." || clean == ".." || len(clean) >= 3 && clean[:3] == "../" {
		return fmt.Errorf("validate bundle path %q: must be relative and stay inside bundle", path)
	}
	return nil
}

func parseFileMode(mode string) (os.FileMode, error) {
	if mode == "" {
		return 0o644, nil
	}
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("parse file mode %q: %w", mode, err)
	}
	return os.FileMode(parsed), nil
}
