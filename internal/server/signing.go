package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ruichen/config-hub/internal/sign"
)

func (s *Server) ensureSigningKey(stateDir string) error {
	if stateDir == "" {
		stateDir = filepath.Join(s.rootDir, "state")
	}
	path := filepath.Join(stateDir, "signing-key.json")
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat signing key: %w", err)
		}
		kp, err := sign.Generate()
		if err != nil {
			return err
		}
		if err := kp.Save(path); err != nil {
			return err
		}
	}
	kp, err := sign.Load(path)
	if err != nil {
		return err
	}
	s.keypair = kp
	return nil
}
