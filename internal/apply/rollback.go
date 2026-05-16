package apply

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RollbackOptions struct {
	RootDir   string
	StateDir  string
	HomeDir   string
	ProfileID string
	Yes       bool
}

func Rollback(opts RollbackOptions) (*Result, error) {
	if !opts.Yes {
		return nil, fmt.Errorf("confirmation required; rerun with --yes to rollback")
	}
	if opts.StateDir == "" {
		opts.StateDir = filepath.Join(opts.RootDir, "state")
	}
	pointer, err := ReadRollbackPointer(opts.StateDir, opts.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("read rollback pointer: %w", err)
	}
	entry, err := lastApplyLogFor(opts.StateDir, opts.ProfileID, pointer.BundleVersion)
	if err != nil {
		return nil, err
	}
	result := &Result{ProfileID: opts.ProfileID, BundleVersion: pointer.BundleVersion, BackupDir: pointer.BackupDir}

	restore := func(action FileAction) error {
		if action.BackupRelPath == "" {
			if action.Action == ActionIncludedOnce {
				data, err := os.ReadFile(action.TargetPath)
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return fmt.Errorf("read target for marker rollback %q: %w", action.TargetPath, err)
				}
				updated, err := RemoveManagedSection(data, MarkerName(action.TemplateID))
				if err != nil {
					return err
				}
				mode := os.FileMode(0o600)
				if info, statErr := os.Stat(action.TargetPath); statErr == nil {
					mode = info.Mode().Perm()
				}
				return AtomicWrite(action.TargetPath, updated, mode)
			}
			if action.Action == ActionWrote || action.Action == ActionManagedSectionUpdate {
				if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("remove newly-created target %q: %w", action.TargetPath, err)
				}
			}
			return nil
		}
		backupPath := filepath.Join(pointer.BackupDir, filepath.FromSlash(action.BackupRelPath))
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read backup %q: %w", backupPath, err)
		}
		info, err := os.Stat(backupPath)
		if err != nil {
			return fmt.Errorf("stat backup %q: %w", backupPath, err)
		}
		if err := AtomicWrite(action.TargetPath, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("restore %q: %w", action.TargetPath, err)
		}
		return nil
	}

	rollbackFiles := make([]FileAction, 0, len(entry.Files))
	for _, action := range entry.Files {
		if action.Action == ActionUnchanged {
			continue
		}
		if err := restore(action); err != nil {
			return nil, err
		}
		action.Action = "rollback"
		rollbackFiles = append(rollbackFiles, action)
	}
	rollbackRemoved := make([]FileAction, 0, len(entry.RemovedFiles))
	for _, action := range entry.RemovedFiles {
		if action.Action == ActionRemovedNoop {
			continue
		}
		if err := restore(action); err != nil {
			return nil, err
		}
		action.Action = "rollback"
		rollbackRemoved = append(rollbackRemoved, action)
	}
	result.Files = rollbackFiles
	result.RemovedFiles = rollbackRemoved
	if err := AppendLog(opts.StateDir, LogEntry{AppliedAt: nowRFC3339(), ProfileID: opts.ProfileID, BundleVersion: pointer.BundleVersion, BackupDir: pointer.BackupDir, Files: rollbackFiles, RemovedFiles: rollbackRemoved}); err != nil {
		return nil, err
	}
	return result, nil
}

func lastApplyLogFor(stateDir, profileID, bundleVersion string) (*LogEntry, error) {
	f, err := os.Open(filepath.Join(stateDir, "apply.log"))
	if err != nil {
		return nil, fmt.Errorf("open apply log: %w", err)
	}
	defer f.Close()
	var last *LogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("parse apply log: %w", err)
		}
		if entry.ProfileID == profileID && entry.BundleVersion == bundleVersion {
			copy := entry
			last = &copy
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read apply log: %w", err)
	}
	if last == nil {
		return nil, fmt.Errorf("no apply log entry for profile %q bundle %q", profileID, bundleVersion)
	}
	return last, nil
}
