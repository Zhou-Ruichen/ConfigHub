package apply

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ruichen/config-hub/internal/bundle"
)

func Apply(ctx context.Context, opts Options) (*Result, error) {
	if opts.StateDir == "" {
		opts.StateDir = filepath.Join(opts.RootDir, "state")
	}
	release, err := AcquireApplyLock(opts.StateDir, opts.ProfileID)
	if err != nil {
		return nil, err
	}
	defer release()

	manifest, files, err := bundle.LoadBundle(opts.BundleDir)
	if err != nil {
		return nil, err
	}
	profileID := opts.ProfileID
	if profileID == "" {
		profileID = manifest.ProfileID
	}
	if profileID != manifest.ProfileID {
		return nil, fmt.Errorf("bundle profile %q does not match active profile %q", manifest.ProfileID, profileID)
	}

	planned, removed, diff, err := PlanApply(ctx, opts.HomeDir, manifest, files)
	if err != nil {
		return nil, err
	}
	result := &Result{ProfileID: profileID, BundleVersion: manifest.BundleVersion, DryRun: opts.DryRun, Files: planned, RemovedFiles: removed, Diff: diff}
	if opts.Out != nil {
		fmt.Fprint(opts.Out, diff)
	}
	if opts.DryRun {
		return result, nil
	}
	if !opts.Yes {
		return result, fmt.Errorf("confirmation required; rerun with --yes to apply")
	}
	if !hasMutations(planned, removed) {
		return result, nil
	}

	backupDir, err := CreateBackupDir(opts.HomeDir, manifest.BundleVersion)
	if err != nil {
		return nil, err
	}
	result.BackupDir = backupDir

	for i := range planned {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if planned[i].Action == ActionUnchanged {
			continue
		}
		rel, err := BackupFile(backupDir, planned[i].TargetPath)
		if err != nil {
			return nil, err
		}
		planned[i].BackupRelPath = rel
		entry := manifest.Files[i]
		content, err := finalContent(opts.HomeDir, entry, files[entry.BundlePath], planned[i].TargetPath)
		if err != nil {
			return nil, err
		}
		mode, err := parseMode(entry.Mode)
		if err != nil {
			return nil, err
		}
		if _, err := WriteTarget(opts.HomeDir, planned[i].TargetPath, entry.Safety.Symlink, content, mode); err != nil {
			return nil, err
		}
	}
	for i := range removed {
		if removed[i].Action == ActionRemovedNoop {
			continue
		}
		rel, err := BackupFile(backupDir, removed[i].TargetPath)
		if err != nil {
			return nil, err
		}
		removed[i].BackupRelPath = rel
		if err := os.Remove(removed[i].TargetPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove target %q: %w", removed[i].TargetPath, err)
		}
	}
	result.Files = planned
	result.RemovedFiles = removed

	appliedAt := nowRFC3339()
	if err := WriteRollbackPointer(opts.StateDir, profileID, RollbackPointer{BackupDir: backupDir, BundleVersion: manifest.BundleVersion, AppliedAt: appliedAt}); err != nil {
		return nil, err
	}
	if err := AppendLog(opts.StateDir, LogEntry{AppliedAt: appliedAt, ProfileID: profileID, BundleVersion: manifest.BundleVersion, BackupDir: backupDir, Files: planned, RemovedFiles: removed}); err != nil {
		return nil, err
	}
	return result, nil
}

func PlanApply(ctx context.Context, homeDir string, manifest *bundle.Manifest, files map[string][]byte) ([]FileAction, []FileAction, string, error) {
	planned := make([]FileAction, 0, len(manifest.Files))
	var diff strings.Builder
	for _, entry := range manifest.Files {
		if err := ctx.Err(); err != nil {
			return nil, nil, "", err
		}
		resolved, err := ValidateTargetPath(homeDir, entry.TargetPath)
		if err != nil {
			return nil, nil, "", err
		}
		if err := validateSafety(entry, resolved); err != nil {
			return nil, nil, "", err
		}
		if err := validateSymlinkForPlan(resolved, entry.Safety.Symlink); err != nil {
			return nil, nil, "", err
		}
		existing, existed, err := readExisting(resolved)
		if err != nil {
			return nil, nil, "", err
		}
		var previous *string
		if existed {
			v := Checksum(existing)
			previous = &v
		}
		content, action, err := plannedContentAndAction(entry, existing, existed, files[entry.BundlePath])
		if err != nil {
			return nil, nil, "", err
		}
		planned = append(planned, FileAction{TemplateID: entry.TemplateID, TargetPath: resolved, Checksum: entry.Checksum, Action: action, PreviousChecksum: previous})
		diff.WriteString(RenderDiff(resolved, existing, content))
	}

	removed := make([]FileAction, 0, len(manifest.RemovedFiles))
	for _, entry := range manifest.RemovedFiles {
		resolved, err := ValidateTargetPath(homeDir, entry.TargetPath)
		if err != nil {
			return nil, nil, "", err
		}
		existing, existed, err := readExisting(resolved)
		if err != nil {
			return nil, nil, "", err
		}
		action := ActionRemovedNoop
		var previous *string
		if existed {
			v := Checksum(existing)
			previous = &v
			if v != entry.PreviousChecksum {
				return nil, nil, "", fmt.Errorf("%w: removed file %q checksum is %s, want %s", ErrPathPolicy, resolved, v, entry.PreviousChecksum)
			}
			action = ActionRemoved
			diff.WriteString(RenderDiff(resolved, existing, nil))
		} else {
			diff.WriteString(fmt.Sprintf("%s: removal no-op (missing)\n", resolved))
		}
		removed = append(removed, FileAction{TemplateID: entry.TemplateID, TargetPath: resolved, Checksum: "", Action: action, PreviousChecksum: previous})
	}
	return planned, removed, diff.String(), nil
}

func plannedContentAndAction(entry bundle.FileEntry, existing []byte, existed bool, rendered []byte) ([]byte, string, error) {
	merge := entry.Safety.Merge
	if merge == "" {
		merge = "replace"
	}
	if merge == "replace" {
		if existed && bytes.Equal(existing, rendered) {
			return rendered, ActionUnchanged, nil
		}
		return rendered, ActionWrote, nil
	}
	if merge == "deep-merge" {
		return nil, "", fmt.Errorf("deep-merge is not implemented before Slice 4")
	}
	marker := MarkerName(entry.TemplateID)
	merged, err := MergeManagedSection(existing, marker, rendered)
	if err != nil {
		return nil, "", err
	}
	if bytes.Equal(existing, merged) {
		return merged, ActionUnchanged, nil
	}
	present, err := MarkerBlockPresent(existing, marker)
	if err != nil {
		return nil, "", err
	}
	if entry.Safety.IncludeStrategy == "append-once" && !present {
		return merged, ActionIncludedOnce, nil
	}
	return merged, ActionManagedSectionUpdate, nil
}

func finalContent(home string, entry bundle.FileEntry, rendered []byte, resolved string) ([]byte, error) {
	existing, existed, err := readExisting(resolved)
	if err != nil {
		return nil, err
	}
	content, _, err := plannedContentAndAction(entry, existing, existed, rendered)
	return content, err
}

func validateSafety(entry bundle.FileEntry, resolved string) error {
	merge := entry.Safety.Merge
	if merge == "" || merge == "replace" {
		return nil
	}
	if merge == "deep-merge" {
		return fmt.Errorf("deep-merge is not implemented before Slice 4")
	}
	if merge != "managed-section" {
		return fmt.Errorf("unsupported merge strategy %q", merge)
	}
	return ValidateMarkerSupported(resolved)
}

func validateSymlinkForPlan(resolved, policy string) error {
	info, err := os.Lstat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat target: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 && (policy == "" || policy == "reject") {
		return fmt.Errorf("%w: target %q is a symlink", ErrPathPolicy, resolved)
	}
	return nil
}

func readExisting(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read existing target %q: %w", path, err)
	}
	return data, true, nil
}

func parseMode(mode string) (os.FileMode, error) {
	if mode == "" {
		return 0o600, nil
	}
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("parse mode %q: %w", mode, err)
	}
	return os.FileMode(parsed), nil
}

func hasMutations(files, removed []FileAction) bool {
	for _, action := range files {
		if action.Action != ActionUnchanged {
			return true
		}
	}
	for _, action := range removed {
		if action.Action != ActionRemovedNoop {
			return true
		}
	}
	return false
}
