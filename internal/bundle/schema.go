package bundle

import (
	"encoding/json"
	"fmt"
	"os"
)

// SupportedSchemaVersion is the manifest schema version supported by this binary.
const SupportedSchemaVersion = "1"

// LoadManifest reads, decodes, defaults, and validates a manifest JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	return ParseManifest(data)
}

// ParseManifest decodes, defaults, and validates a manifest JSON document.
func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	ApplyDefaults(&manifest)
	if err := Validate(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ApplyDefaults fills defaults in manifest file-entry safety blocks.
func ApplyDefaults(manifest *Manifest) {
	for i := range manifest.Files {
		if manifest.Files[i].Safety.Secrets == "" {
			manifest.Files[i].Safety.Secrets = "forbidden"
		}
	}
}

// Validate checks Slice 1 manifest contract requirements.
func Validate(manifest *Manifest) error {
	if manifest.SchemaVersion != SupportedSchemaVersion {
		return fmt.Errorf("validate manifest: unsupported schemaVersion %q", manifest.SchemaVersion)
	}
	if manifest.BundleVersion == "" {
		return fmt.Errorf("validate manifest: bundleVersion is required")
	}
	if manifest.ProfileID == "" {
		return fmt.Errorf("validate manifest: profileId is required")
	}
	return nil
}
