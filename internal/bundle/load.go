package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrChecksumMismatch = errors.New("checksum mismatch")

// LoadBundle reads manifest.json, checksums.json, and every rendered file named
// by manifest.files[]. It verifies each rendered file against both the manifest
// checksum and the checksums.json entry before returning bytes to the apply path.
func LoadBundle(bundleDir string) (*Manifest, map[string][]byte, error) {
	manifest, err := LoadManifest(filepath.Join(bundleDir, "manifest.json"))
	if err != nil {
		return nil, nil, err
	}

	checksumsData, err := os.ReadFile(filepath.Join(bundleDir, "checksums.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("read checksums.json: %w", err)
	}
	var checksums map[string]string
	if err := json.Unmarshal(checksumsData, &checksums); err != nil {
		return nil, nil, fmt.Errorf("parse checksums.json: %w", err)
	}

	files := make(map[string][]byte, len(manifest.Files))
	for _, entry := range manifest.Files {
		if err := validateBundlePath(entry.BundlePath); err != nil {
			return nil, nil, err
		}
		path := filepath.Join(bundleDir, filepath.FromSlash(entry.BundlePath))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read rendered file %q: %w", entry.BundlePath, err)
		}
		actual := sha256Checksum(data)
		if actual != entry.Checksum {
			return nil, nil, fmt.Errorf("%w: %s manifest has %s, file has %s", ErrChecksumMismatch, entry.BundlePath, entry.Checksum, actual)
		}
		if checksums[entry.BundlePath] != actual {
			return nil, nil, fmt.Errorf("%w: %s checksums.json has %s, file has %s", ErrChecksumMismatch, entry.BundlePath, checksums[entry.BundlePath], actual)
		}
		files[entry.BundlePath] = data
	}
	return manifest, files, nil
}

func sha256Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
