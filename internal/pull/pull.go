package pull

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/sign"
)

type PullOptions struct {
	DryRun bool
	Client *Client
}

type PullResult struct {
	Profile       string `json:"profile"`
	BundleVersion string `json:"bundleVersion"`
	Path          string `json:"path,omitempty"`
	DryRun        bool   `json:"dryRun"`
}

func Pull(ctx context.Context, cfg *HubConfig, stateDir string, opts PullOptions) (*PullResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("pull: nil hub config")
	}
	client := opts.Client
	if client == nil {
		client = New(cfg)
	}
	serverPub, err := client.FetchSigningKey(ctx)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(serverPub, cfg.PinnedPublicKey) {
		return nil, ErrPinnedKeyMismatch
	}
	manifest, err := client.FetchManifest(ctx)
	if err != nil {
		return nil, err
	}
	if manifest.SchemaVersion != bundle.SupportedSchemaVersion {
		return nil, ErrSchemaUnsupported
	}
	if manifest.Signature == nil || manifest.Signature.Algorithm != sign.AlgorithmEd25519 {
		return nil, ErrSignatureAlgorithm
	}
	if err := sign.VerifyManifest(manifest, manifest.Signature, cfg.PinnedPublicKey); err != nil {
		if errors.Is(err, sign.ErrSignatureAlgorithm) {
			return nil, ErrSignatureAlgorithm
		}
		if errors.Is(err, sign.ErrSignatureInvalid) {
			return nil, ErrSignatureInvalid
		}
		return nil, err
	}
	if manifest.ProfileID != cfg.Profile {
		return nil, ErrProfileMismatch
	}

	body, err := client.FetchBundle(ctx)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	archive, err := io.ReadAll(io.LimitReader(body, maxBundleBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read bundle archive: %w", err)
	}
	if int64(len(archive)) > maxBundleBytes {
		return nil, fmt.Errorf("bundle archive exceeds %d bytes", maxBundleBytes)
	}

	profileDir := filepath.Join(stateDir, "pull", cfg.Profile)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, fmt.Errorf("create pull profile dir: %w", err)
	}
	_ = os.Chmod(filepath.Join(stateDir, "pull"), 0o700)
	_ = os.Chmod(profileDir, 0o700)
	tmpDir, err := os.MkdirTemp(profileDir, ".tmp-")
	if err != nil {
		return nil, fmt.Errorf("create pull tmp dir: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := os.WriteFile(filepath.Join(tmpDir, "bundle.tar.gz"), archive, 0o600); err != nil {
		return nil, fmt.Errorf("write bundle archive: %w", err)
	}
	if err := extractTarGz(archive, tmpDir); err != nil {
		return nil, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode verified manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), manifestData, 0o600); err != nil {
		return nil, fmt.Errorf("write verified manifest: %w", err)
	}
	loaded, _, err := bundle.LoadBundle(tmpDir)
	if err != nil {
		return nil, err
	}
	if loaded.ProfileID != cfg.Profile {
		return nil, ErrProfileMismatch
	}
	if opts.DryRun {
		return &PullResult{Profile: cfg.Profile, BundleVersion: manifest.BundleVersion, Path: filepath.Join(profileDir, manifest.BundleVersion), DryRun: true}, nil
	}

	finalDir := filepath.Join(profileDir, manifest.BundleVersion)
	if err := os.RemoveAll(finalDir); err != nil {
		return nil, fmt.Errorf("remove existing pull bundle: %w", err)
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return nil, fmt.Errorf("install pulled bundle: %w", err)
	}
	cleanup = false
	if err := updateLatest(profileDir, manifest.BundleVersion); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	cfg.LastSyncAt = &now
	if err := cfg.Save(stateDir); err != nil {
		return nil, err
	}
	return &PullResult{Profile: cfg.Profile, BundleVersion: manifest.BundleVersion, Path: finalDir, DryRun: false}, nil
}

func extractTarGz(data []byte, dst string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open bundle archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read bundle archive: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			return fmt.Errorf("bundle archive contains non-regular entry %q", h.Name)
		}
		clean := filepath.Clean(filepath.FromSlash(h.Name))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || filepath.IsAbs(h.Name) {
			return fmt.Errorf("bundle archive entry escapes destination %q", h.Name)
		}
		path := filepath.Join(dst, clean)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create archive directory: %w", err)
		}
		mode := os.FileMode(h.Mode).Perm()
		if mode == 0 {
			mode = 0o600
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("create archive file %q: %w", h.Name, err)
		}
		_, copyErr := io.Copy(f, tr)
		closeErr := f.Close()
		if copyErr != nil {
			return fmt.Errorf("write archive file %q: %w", h.Name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close archive file %q: %w", h.Name, closeErr)
		}
	}
	return nil
}
