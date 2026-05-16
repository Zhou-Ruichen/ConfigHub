package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ruichen/config-hub/internal/bundle"
)

type DoctorOptions struct {
	RootDir   string
	StateDir  string
	HomeDir   string
	ProfileID string
	BundleDir string
}

type DoctorResult struct {
	OK     bool     `json:"ok"`
	Checks []string `json:"checks"`
	Errors []string `json:"errors"`
}

func DoctorApply(ctx context.Context, opts DoctorOptions) (*DoctorResult, error) {
	res := &DoctorResult{OK: true}
	addErr := func(format string, args ...any) {
		res.OK = false
		res.Errors = append(res.Errors, fmt.Sprintf(format, args...))
	}
	if opts.StateDir == "" {
		opts.StateDir = filepath.Join(opts.RootDir, "state")
	}
	if err := os.MkdirAll(opts.HomeDir, 0o700); err != nil {
		addErr("home dir not writable: %v", err)
	} else {
		test := filepath.Join(opts.HomeDir, ".confighub-doctor")
		if err := os.WriteFile(test, []byte("ok"), 0o600); err != nil {
			addErr("home dir not writable: %v", err)
		} else {
			_ = os.Remove(test)
			res.Checks = append(res.Checks, "home dir writable")
		}
	}
	if dir, err := CreateBackupDir(opts.HomeDir, ".doctor"); err != nil {
		addErr("backup dir not creatable: %v", err)
	} else {
		_ = os.RemoveAll(dir)
		res.Checks = append(res.Checks, "backup dir creatable")
	}
	if ok, err := hasFreeSpace(opts.HomeDir, 100<<20); err != nil {
		addErr("free disk space check failed: %v", err)
	} else if !ok {
		addErr("free disk space below 100 MiB")
	} else {
		res.Checks = append(res.Checks, "free disk space >= 100 MiB")
	}
	if _, err := os.Stat(filepath.Join(opts.StateDir, "apply.lock")); err == nil {
		if _, err := AcquireApplyLock(opts.StateDir, opts.ProfileID); err != nil {
			addErr("apply lock active or stale: %v", err)
		}
	}
	if opts.BundleDir != "" {
		manifest, files, err := bundle.LoadBundle(opts.BundleDir)
		if err != nil {
			addErr("bundle invalid: %v", err)
		} else if _, _, _, err := PlanApply(ctx, opts.HomeDir, manifest, files); err != nil {
			addErr("target validation failed: %v", err)
		} else {
			for _, entry := range manifest.Files {
				resolved, _ := ValidateTargetPath(opts.HomeDir, entry.TargetPath)
				if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
					addErr("target dir %q not creatable: %v", filepath.Dir(resolved), err)
				}
			}
			res.Checks = append(res.Checks, "target dirs resolvable")
		}
	}
	if !res.OK {
		return res, fmt.Errorf("doctor apply failed")
	}
	return res, nil
}

func hasFreeSpace(path string, min uint64) (bool, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return false, err
	}
	return st.Bavail*uint64(st.Bsize) >= min, nil
}
