package pull

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/ruichen/config-hub/internal/bundle"
)

// Response-size caps protect the client from a malicious or misconfigured hub
// returning oversized payloads. Manifest signature catches tampering of valid
// payloads; these caps stop the client from OOMing before signature verify runs.
const (
	maxSigningKeyBytes = 4 << 10   // 4 KiB
	maxManifestBytes   = 1 << 20   // 1 MiB
	maxBundleBytes     = 100 << 20 // 100 MiB
)

type Client struct {
	http *http.Client
	cfg  *HubConfig
}

func New(cfg *HubConfig) *Client {
	return &Client{http: &http.Client{Timeout: 120 * time.Second}, cfg: cfg}
}

func NewWithHTTPClient(cfg *HubConfig, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{http: hc, cfg: cfg}
}

func (c *Client) FetchSigningKey(ctx context.Context) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/signing-key", false)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch signing key: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSigningKeyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read signing key response: %w", err)
	}
	if int64(len(data)) > maxSigningKeyBytes {
		return nil, fmt.Errorf("signing key response exceeds %d bytes", maxSigningKeyBytes)
	}
	var body struct {
		Algorithm string `json:"algorithm"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse signing key response: %w", err)
	}
	if body.Algorithm != "ed25519" {
		return nil, ErrSignatureAlgorithm
	}
	pub, err := base64.StdEncoding.DecodeString(body.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode signing key response: %w", err)
	}
	return pub, nil
}

func (c *Client) FetchManifest(ctx context.Context) (*bundle.Manifest, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path.Join("/api/v1/profiles", c.cfg.Profile, "bundle/manifest"), true)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read manifest response: %w", err)
	}
	if int64(len(data)) > maxManifestBytes {
		return nil, fmt.Errorf("manifest response exceeds %d bytes", maxManifestBytes)
	}
	var manifest bundle.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManifestUnparseable, err)
	}
	bundle.ApplyDefaults(&manifest)
	if manifest.Domains == nil {
		manifest.Domains = []string{}
	}
	if manifest.Files == nil {
		manifest.Files = []bundle.FileEntry{}
	}
	if manifest.RemovedFiles == nil {
		manifest.RemovedFiles = []bundle.RemovedFileEntry{}
	}
	return &manifest, nil
}

func (c *Client) FetchBundle(ctx context.Context) (io.ReadCloser, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path.Join("/api/v1/profiles", c.cfg.Profile, "bundle"), true)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch bundle: %w", err)
	}
	if err := checkStatus(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) newRequest(ctx context.Context, method, suffix string, auth bool) (*http.Request, error) {
	base, err := url.Parse(c.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse hub URL: %w", err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + suffix
	req, err := http.NewRequestWithContext(ctx, method, base.String(), nil)
	if err != nil {
		return nil, err
	}
	if auth && c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	return req, nil
}

func checkStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return ErrHTTPAuth
	case http.StatusForbidden:
		return ErrHTTPAuth
	case http.StatusNotFound:
		return ErrHTTPNotFound
	default:
		return fmt.Errorf("hub returned status %d", resp.StatusCode)
	}
}
