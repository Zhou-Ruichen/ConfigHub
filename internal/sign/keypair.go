package sign

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const AlgorithmEd25519 = "ed25519"

type Keypair struct {
	Algorithm  string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	CreatedAt  time.Time
}

type keypairFile struct {
	Algorithm  string `json:"algorithm"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
	CreatedAt  string `json:"createdAt"`
}

func Generate() (*Keypair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing keypair: %w", err)
	}
	return &Keypair{Algorithm: AlgorithmEd25519, PublicKey: pub, PrivateKey: priv, CreatedAt: time.Now().UTC()}, nil
}

func Load(path string) (*Keypair, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat signing key %q: %w", path, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("signing key %q permissions %o exceed 0600", path, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read signing key %q: %w", path, err)
	}
	var raw keypairFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse signing key %q: %w", path, err)
	}
	if raw.Algorithm != AlgorithmEd25519 {
		return nil, fmt.Errorf("signing key %q algorithm %q: %w", path, raw.Algorithm, ErrSignatureAlgorithm)
	}
	pub, err := base64.StdEncoding.DecodeString(raw.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode signing public key: %w", err)
	}
	priv, err := base64.StdEncoding.DecodeString(raw.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode signing private key: %w", err)
	}
	if l := len(pub); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("decode signing public key: length %d, want %d", l, ed25519.PublicKeySize)
	}
	if l := len(priv); l != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("decode signing private key: length %d, want %d", l, ed25519.PrivateKeySize)
	}
	created, err := time.Parse(time.RFC3339, raw.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse signing key createdAt: %w", err)
	}
	return &Keypair{Algorithm: raw.Algorithm, PublicKey: ed25519.PublicKey(pub), PrivateKey: ed25519.PrivateKey(priv), CreatedAt: created.UTC()}, nil
}

func (k *Keypair) Save(path string) error {
	if k == nil {
		return fmt.Errorf("save signing key: nil keypair")
	}
	if k.Algorithm == "" {
		k.Algorithm = AlgorithmEd25519
	}
	if k.Algorithm != AlgorithmEd25519 {
		return fmt.Errorf("save signing key: %w", ErrSignatureAlgorithm)
	}
	if len(k.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("save signing key: public key length %d", len(k.PublicKey))
	}
	if len(k.PrivateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("save signing key: private key length %d", len(k.PrivateKey))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create signing key directory: %w", err)
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)
	created := k.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	raw := keypairFile{
		Algorithm:  k.Algorithm,
		PublicKey:  base64.StdEncoding.EncodeToString(k.PublicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(k.PrivateKey),
		CreatedAt:  created.Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode signing key: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create temp signing key: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(path + ".tmp")
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp signing key: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp signing key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp signing key: %w", err)
	}
	// os.Link (not os.Rename) is intentional: rotation is out of scope for now,
	// and Link fails atomically with EEXIST if the destination already exists,
	// implementing "refuse to overwrite" without a TOCTOU stat-then-rename.
	if err := os.Link(path+".tmp", path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("save signing key: refusing to overwrite %q", path)
		}
		return fmt.Errorf("install signing key: %w", err)
	}
	cleanup = false
	_ = os.Remove(path + ".tmp")
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
