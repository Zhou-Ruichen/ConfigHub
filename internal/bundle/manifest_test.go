package bundle

import (
	"encoding/json"
	"testing"
)

const fixtureManifestPath = "../../examples/bundles/macbook/2026-05-16T15-04-22Z-001/manifest.json"

func TestLoadManifest(t *testing.T) {
	tests := []struct {
		name    string
		load    func(t *testing.T) (*Manifest, error)
		wantErr bool
		check   func(t *testing.T, manifest *Manifest)
	}{
		{
			name: "load fixture manifest",
			load: func(t *testing.T) (*Manifest, error) {
				t.Helper()
				return LoadManifest(fixtureManifestPath)
			},
			check: func(t *testing.T, manifest *Manifest) {
				t.Helper()
				if manifest.SchemaVersion != SupportedSchemaVersion {
					t.Fatalf("SchemaVersion = %q, want %q", manifest.SchemaVersion, SupportedSchemaVersion)
				}
				if manifest.Signature != nil {
					t.Fatalf("Signature = %#v, want nil", manifest.Signature)
				}
				if len(manifest.RemovedFiles) != 1 {
					t.Fatalf("len(RemovedFiles) = %d, want 1", len(manifest.RemovedFiles))
				}
				removed := manifest.RemovedFiles[0]
				if removed.TemplateID != "ai/gemini-legacy" || removed.Reason != "removed" || removed.TargetPath != "~/.config/gemini/old.json" {
					t.Fatalf("RemovedFiles[0] = %#v", removed)
				}
			},
		},
		{
			name: "unsupported schema rejected",
			load: func(t *testing.T) (*Manifest, error) {
				t.Helper()
				return ParseManifest([]byte(`{
  "schemaVersion": "999",
  "bundleVersion": "2026-05-16T15-04-22Z-001",
  "profileId": "macbook",
  "createdAt": "2026-05-16T15:04:22Z",
  "sourceRevision": "git:abc1234",
  "domains": [],
  "files": [],
  "removedFiles": [],
  "changeSummary": "",
  "signature": null
}`))
			},
			wantErr: true,
		},
		{
			name: "signature null accepted",
			load: func(t *testing.T) (*Manifest, error) {
				t.Helper()
				return ParseManifest([]byte(`{
  "schemaVersion": "1",
  "bundleVersion": "2026-05-16T15-04-22Z-001",
  "profileId": "macbook",
  "createdAt": "2026-05-16T15:04:22Z",
  "sourceRevision": "git:abc1234",
  "domains": [],
  "files": [],
  "removedFiles": [],
  "changeSummary": "",
  "signature": null
}`))
			},
			check: func(t *testing.T, manifest *Manifest) {
				t.Helper()
				if manifest.Signature != nil {
					t.Fatalf("Signature = %#v, want nil", manifest.Signature)
				}
			},
		},
		{
			name: "default file safety secrets forbidden",
			load: func(t *testing.T) (*Manifest, error) {
				t.Helper()
				return ParseManifest([]byte(`{
  "schemaVersion": "1",
  "bundleVersion": "2026-05-16T15-04-22Z-001",
  "profileId": "macbook",
  "createdAt": "2026-05-16T15:04:22Z",
  "sourceRevision": "git:abc1234",
  "domains": ["dotfiles"],
  "files": [
    {
      "templateId": "dotfiles/git",
      "domain": "dotfiles",
      "bundlePath": "files/dotfiles/git/confighub.gitconfig",
      "targetPath": "~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig",
      "mode": "0644",
      "checksum": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
      "delivery": "sync",
      "safety": {
        "backup": "required",
        "diff": "required",
        "symlink": "reject",
        "merge": "replace"
      }
    }
  ],
  "removedFiles": [],
  "changeSummary": "",
  "signature": null
}`))
			},
			check: func(t *testing.T, manifest *Manifest) {
				t.Helper()
				if got := manifest.Files[0].Safety.Secrets; got != "forbidden" {
					t.Fatalf("Safety.Secrets = %q, want forbidden", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := tt.load(t)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("LoadManifest() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadManifest() error = %v", err)
			}
			if tt.check != nil {
				tt.check(t, manifest)
			}
		})
	}
}

func TestManifestJSONRoundTrip(t *testing.T) {
	manifest, err := LoadManifest(fixtureManifestPath)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	roundTrip, err := ParseManifest(encoded)
	if err != nil {
		t.Fatalf("ParseManifest(roundTrip) error = %v", err)
	}
	if len(roundTrip.Files) != 3 {
		t.Fatalf("len(Files) = %d, want 3", len(roundTrip.Files))
	}
}
