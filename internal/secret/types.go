package secret

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Reference is reserved for future secret reference metadata.
type Reference struct{}

// Token is the persisted token metadata. Plaintext tokens are never stored.
type Token struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Scope     string `json:"scope"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"createdAt"`
}

var (
	ErrUnknownToken  = errors.New("unknown token")
	ErrInvalidScope  = errors.New("invalid token scope")
	ErrTokenNotFound = errors.New("token not found")
)

// Create generates a plaintext bearer token, persists only its hash and
// metadata under rootDir/state/tokens, and returns the plaintext exactly once.
func Create(rootDir, label, scope string) (string, *Token, error) {
	if err := ValidateScope(scope); err != nil {
		return "", nil, err
	}
	plaintextBytes := make([]byte, 32)
	if _, err := rand.Read(plaintextBytes); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	idBytes := make([]byte, 9)
	if _, err := rand.Read(idBytes); err != nil {
		return "", nil, fmt.Errorf("generate token id: %w", err)
	}

	plaintext := "cfh_" + base64.RawURLEncoding.EncodeToString(plaintextBytes)
	id := "cfh_" + base64.RawURLEncoding.EncodeToString(idBytes)
	tok := &Token{
		ID:        id,
		Label:     label,
		Scope:     scope,
		Hash:      HashPlaintext(plaintext),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeToken(rootDir, tok); err != nil {
		return "", nil, err
	}
	return plaintext, tok, nil
}

// List returns persisted token metadata sorted by id. It never returns plaintext.
func List(rootDir string) ([]Token, error) {
	dir := tokensDir(rootDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Token{}, nil
		}
		return nil, fmt.Errorf("read tokens directory: %w", err)
	}
	tokens := make([]Token, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read token %q: %w", entry.Name(), err)
		}
		var tok Token
		if err := json.Unmarshal(data, &tok); err != nil {
			return nil, fmt.Errorf("parse token %q: %w", entry.Name(), err)
		}
		tokens = append(tokens, tok)
	}
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].ID < tokens[j].ID })
	return tokens, nil
}

// Lookup hashes plaintext and compares it with stored hashes using constant-time
// comparison. ErrUnknownToken is returned when no stored hash matches.
func Lookup(rootDir, plaintext string) (*Token, error) {
	candidate := HashPlaintext(plaintext)
	tokens, err := List(rootDir)
	if err != nil {
		return nil, err
	}
	for i := range tokens {
		if hmac.Equal([]byte(tokens[i].Hash), []byte(candidate)) {
			return &tokens[i], nil
		}
	}
	return nil, ErrUnknownToken
}

// Revoke removes a persisted token by id.
func Revoke(rootDir, id string) error {
	path := filepath.Join(tokensDir(rootDir), id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("remove token %q: %w", id, err)
	}
	return nil
}

// ValidateScope checks the Slice 4 token scope grammar.
func ValidateScope(scope string) error {
	if scope == "admin" {
		return nil
	}
	if strings.HasPrefix(scope, "pull:") && strings.TrimPrefix(scope, "pull:") != "" {
		return nil
	}
	if strings.HasPrefix(scope, "read:") && strings.TrimPrefix(scope, "read:") != "" {
		return nil
	}
	return ErrInvalidScope
}

// HashPlaintext returns the persisted sha256 hash form for a plaintext token.
func HashPlaintext(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func tokensDir(rootDir string) string {
	return filepath.Join(rootDir, "state", "tokens")
}

func writeToken(rootDir string, tok *Token) error {
	stateDir := filepath.Join(rootDir, "state")
	dir := tokensDir(rootDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}
	_ = os.Chmod(stateDir, 0o700)
	_ = os.Chmod(dir, 0o700)

	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("encode token metadata: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, tok.ID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create token temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod token temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write token temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync token temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close token temp file: %w", err)
	}
	finalPath := filepath.Join(dir, tok.ID+".json")
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("promote token file: %w", err)
	}
	cleanup = false
	_ = os.Chmod(finalPath, 0o600)
	if parent, err := os.Open(dir); err == nil {
		_ = parent.Sync()
		_ = parent.Close()
	}
	return nil
}
