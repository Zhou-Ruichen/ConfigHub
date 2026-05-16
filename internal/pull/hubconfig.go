package pull

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type HubConfig struct {
	URL             string
	Profile         string
	Token           string
	PinnedPublicKey []byte
	SchemaVersion   string
	LastSyncAt      *time.Time
}

type hubConfigFile struct {
	URL             string  `json:"url"`
	Profile         string  `json:"profile"`
	Token           string  `json:"token"`
	PinnedPublicKey string  `json:"pinnedPublicKey"`
	SchemaVersion   string  `json:"schemaVersion"`
	LastSyncAt      *string `json:"lastSyncAt"`
}

func Load(stateDir string) (*HubConfig, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, "hub.json"))
	if err != nil {
		return nil, fmt.Errorf("read hub config: %w", err)
	}
	var raw hubConfigFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse hub config: %w", err)
	}
	pub, err := base64.StdEncoding.DecodeString(raw.PinnedPublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode pinned public key: %w", err)
	}
	var last *time.Time
	if raw.LastSyncAt != nil && *raw.LastSyncAt != "" {
		parsed, err := time.Parse(time.RFC3339, *raw.LastSyncAt)
		if err != nil {
			return nil, fmt.Errorf("parse hub lastSyncAt: %w", err)
		}
		utc := parsed.UTC()
		last = &utc
	}
	return &HubConfig{URL: raw.URL, Profile: raw.Profile, Token: raw.Token, PinnedPublicKey: pub, SchemaVersion: raw.SchemaVersion, LastSyncAt: last}, nil
}

func (c *HubConfig) Save(stateDir string) error {
	if c == nil {
		return fmt.Errorf("save hub config: nil config")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	_ = os.Chmod(stateDir, 0o700)
	var last *string
	if c.LastSyncAt != nil {
		s := c.LastSyncAt.UTC().Format(time.RFC3339)
		last = &s
	}
	raw := hubConfigFile{
		URL:             c.URL,
		Profile:         c.Profile,
		Token:           c.Token,
		PinnedPublicKey: base64.StdEncoding.EncodeToString(c.PinnedPublicKey),
		SchemaVersion:   c.SchemaVersion,
		LastSyncAt:      last,
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode hub config: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(stateDir, "hub.json")
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create temp hub config: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp hub config: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync temp hub config: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp hub config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("install hub config: %w", err)
	}
	cleanup = false
	if dir, err := os.Open(stateDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
