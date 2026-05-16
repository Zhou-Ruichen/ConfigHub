package apply

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type StatusOptions struct {
	RootDir   string
	StateDir  string
	ProfileID string
}

type StatusResult struct {
	SchemaVersion     string `json:"schemaVersion"`
	ActiveProfile     string `json:"activeProfile"`
	LastBundleVersion string `json:"lastBundleVersion,omitempty"`
	LastAppliedAt     string `json:"lastAppliedAt,omitempty"`
	LastBackupDir     string `json:"lastBackupDir,omitempty"`
}

func Status(opts StatusOptions) (*StatusResult, error) {
	if opts.StateDir == "" {
		opts.StateDir = filepath.Join(opts.RootDir, "state")
	}
	profileID := opts.ProfileID
	if profileID == "" {
		data, err := os.ReadFile(filepath.Join(opts.StateDir, "active-profile"))
		if err != nil {
			return nil, err
		}
		profileID = strings.TrimSpace(string(data))
	}
	res := &StatusResult{SchemaVersion: "1", ActiveProfile: profileID}
	if pointer, err := ReadRollbackPointer(opts.StateDir, profileID); err == nil {
		res.LastBundleVersion = pointer.BundleVersion
		res.LastAppliedAt = pointer.AppliedAt
		res.LastBackupDir = pointer.BackupDir
	}
	if last, err := lastLogForProfile(opts.StateDir, profileID); err == nil && last != nil {
		res.LastBundleVersion = last.BundleVersion
		res.LastAppliedAt = last.AppliedAt
		res.LastBackupDir = last.BackupDir
	}
	return res, nil
}

func FormatStatus(res *StatusResult) string {
	return fmt.Sprintf("active profile: %s\nlast bundle version: %s\nlast applied at: %s\nlast backup dir: %s\n", res.ActiveProfile, res.LastBundleVersion, res.LastAppliedAt, res.LastBackupDir)
}

func lastLogForProfile(stateDir, profileID string) (*LogEntry, error) {
	f, err := os.Open(filepath.Join(stateDir, "apply.log"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var last *LogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		if entry.ProfileID == profileID {
			copy := entry
			last = &copy
		}
	}
	return last, scanner.Err()
}
