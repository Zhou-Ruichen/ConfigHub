package apply

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LogEntry struct {
	AppliedAt     string       `json:"appliedAt"`
	ProfileID     string       `json:"profileId"`
	BundleVersion string       `json:"bundleVersion"`
	BackupDir     string       `json:"backupDir"`
	Files         []FileAction `json:"files"`
	RemovedFiles  []FileAction `json:"removedFiles"`
}

type RollbackPointer struct {
	BackupDir     string `json:"backupDir"`
	BundleVersion string `json:"bundleVersion"`
	AppliedAt     string `json:"appliedAt"`
}

func AppendLog(stateDir string, entry LogEntry) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.Chmod(stateDir, 0o700); err != nil {
		return fmt.Errorf("chmod state dir: %w", err)
	}
	path := filepath.Join(stateDir, "apply.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open apply log: %w", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("encode apply log entry: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write apply log: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("fsync apply log: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close apply log: %w", err)
	}
	return nil
}

func WriteRollbackPointer(stateDir, profileID string, pointer RollbackPointer) error {
	path := filepath.Join(stateDir, "rollback", profileID+".json")
	data, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return fmt.Errorf("encode rollback pointer: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create rollback dir: %w", err)
	}
	return AtomicWrite(path, data, 0o600)
}

func ReadRollbackPointer(stateDir, profileID string) (*RollbackPointer, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, "rollback", profileID+".json"))
	if err != nil {
		return nil, err
	}
	var pointer RollbackPointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return nil, fmt.Errorf("parse rollback pointer: %w", err)
	}
	return &pointer, nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
