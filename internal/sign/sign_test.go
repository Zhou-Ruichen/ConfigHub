package sign

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/ruichen/config-hub/internal/bundle"
)

func testManifest() *bundle.Manifest {
	return &bundle.Manifest{
		SchemaVersion:  "1",
		BundleVersion:  "v1",
		ProfileID:      "macbook",
		CreatedAt:      "2026-05-16T20:00:00Z",
		SourceRevision: "test",
		Domains:        []string{"dotfiles"},
		Files: []bundle.FileEntry{{
			TemplateID: "dotfiles/git",
			Domain:     "dotfiles",
			BundlePath: "files/gitconfig",
			TargetPath: "~/.gitconfig.local",
			Mode:       "0600",
			Checksum:   "sha256:abc",
			Delivery:   "sync",
			Safety:     bundle.Safety{Backup: "required", Diff: "required", Symlink: "reject", Secrets: "forbidden", Merge: "replace"},
		}},
		RemovedFiles:  []bundle.RemovedFileEntry{},
		ChangeSummary: "test",
	}
}

func TestSignVerifyRoundTripTamperAndWrongKey(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	manifest := testManifest()
	sig, err := SignManifest(manifest, kp.PrivateKey)
	if err != nil {
		t.Fatalf("SignManifest() error = %v", err)
	}
	manifest.Signature = sig
	if err := VerifyManifest(manifest, manifest.Signature, kp.PublicKey); err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}

	manifest.ProfileID = "tampered"
	if err := VerifyManifest(manifest, manifest.Signature, kp.PublicKey); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("tampered VerifyManifest() error = %v, want ErrSignatureInvalid", err)
	}
	manifest.ProfileID = "macbook"

	wrongPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := VerifyManifest(manifest, manifest.Signature, wrongPub); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("wrong key VerifyManifest() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifyManifestRejectsMissingAndWrongAlgorithm(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	manifest := testManifest()
	if err := VerifyManifest(manifest, nil, kp.PublicKey); !errors.Is(err, ErrSignatureAlgorithm) {
		t.Fatalf("nil signature error = %v, want ErrSignatureAlgorithm", err)
	}
	if err := VerifyManifest(manifest, &bundle.Signature{Algorithm: "rsa", Value: "x"}, kp.PublicKey); !errors.Is(err, ErrSignatureAlgorithm) {
		t.Fatalf("wrong algorithm error = %v, want ErrSignatureAlgorithm", err)
	}
}

func TestCanonicalManifestBytesStableAndIgnoresSignature(t *testing.T) {
	manifest := testManifest()
	one, err := CanonicalManifestBytes(manifest)
	if err != nil {
		t.Fatalf("CanonicalManifestBytes() error = %v", err)
	}
	two, err := CanonicalManifestBytes(manifest)
	if err != nil {
		t.Fatalf("CanonicalManifestBytes() error = %v", err)
	}
	if !bytes.Equal(one, two) {
		t.Fatalf("canonical bytes changed between runs")
	}
	manifest.Signature = &bundle.Signature{Algorithm: AlgorithmEd25519, Value: "abc"}
	withSig, err := CanonicalManifestBytes(manifest)
	if err != nil {
		t.Fatalf("CanonicalManifestBytes(with signature) error = %v", err)
	}
	if !bytes.Equal(one, withSig) {
		t.Fatalf("canonical bytes include signature\nwithout=%s\nwith=%s", one, withSig)
	}
	if bytes.Contains(withSig, []byte("\"abc\"")) {
		t.Fatalf("canonical bytes contain signature value: %s", withSig)
	}
}
