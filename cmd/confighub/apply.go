package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ruichen/config-hub/internal/apply"
	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/spf13/cobra"
)

const (
	exitPathPolicy = 11
	exitChecksum   = 12
	exitRollback   = 30
)

func newApplyCommand() *cobra.Command {
	var bundleFlag, profileFlag, rootFlag string
	var yes, dryRun, jsonOut bool
	cmd := &cobra.Command{
		Use:   "apply --bundle <path-or-version> [--profile <id-or-path>] [--root <dir>] [--yes] [--dry-run] [--json]",
		Short: "Back up and apply a rendered bundle",
		Example: "confighub apply --bundle ~/.config/confighub/bundles/macbook/latest --profile macbook --yes\n" +
			"confighub apply --bundle latest --root examples --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("apply does not accept positional arguments")}
			}
			root, profileID, bundleDir, home, err := resolveApplyInputs(rootFlag, profileFlag, bundleFlag, true)
			if err != nil {
				return err
			}
			res, err := apply.Apply(context.Background(), apply.Options{BundleDir: bundleDir, RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: profileID, DryRun: dryRun, Yes: yes, JSON: jsonOut, Out: cmd.OutOrStdout()})
			if err != nil {
				return mapApplyError(err)
			}
			if jsonOut {
				data, _ := json.MarshalIndent(res, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "dry run: no files written")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "applied bundle %s\n", bundleDir)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bundleFlag, "bundle", "", "bundle path, version, or latest")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile id or YAML path")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "compute diff but write nothing")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON result")
	return cmd
}

func newDiffCommand() *cobra.Command {
	var bundleFlag, profileFlag, rootFlag string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "diff --bundle <path-or-version> [--profile <id-or-path>] [--root <dir>] [--json]",
		Short:   "Compare a bundle with local targets",
		Example: "confighub diff --bundle latest --profile macbook --root examples",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("diff does not accept positional arguments")}
			}
			root, profileID, bundleDir, home, err := resolveApplyInputs(rootFlag, profileFlag, bundleFlag, true)
			if err != nil {
				return err
			}
			manifest, files, err := bundle.LoadBundle(bundleDir)
			if err != nil {
				return mapApplyError(err)
			}
			planned, removed, diff, err := apply.PlanApply(context.Background(), home, manifest, files)
			if err != nil {
				return mapApplyError(err)
			}
			if jsonOut {
				data, _ := json.MarshalIndent(apply.Result{ProfileID: profileID, BundleVersion: manifest.BundleVersion, DryRun: true, Files: planned, RemovedFiles: removed, Diff: diff}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), diff)
			_ = root
			return nil
		},
	}
	cmd.Flags().StringVar(&bundleFlag, "bundle", "", "bundle path, version, or latest")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile id or YAML path")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON result")
	return cmd
}

func newStatusCommand() *cobra.Command {
	var profileFlag, rootFlag string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "status [--profile <id-or-path>] [--root <dir>] [--json]",
		Short:   "Show local ConfigHub state",
		Example: "confighub status --profile macbook --root examples",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("status does not accept positional arguments")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			profileID, err := resolveProfileID(root, profileFlag)
			if err != nil {
				return err
			}
			res, err := apply.Status(apply.StatusOptions{RootDir: root, StateDir: filepath.Join(root, "state"), ProfileID: profileID})
			if err != nil {
				return err
			}
			if jsonOut {
				data, _ := json.MarshalIndent(res, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprint(cmd.OutOrStdout(), apply.FormatStatus(res))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile id or YAML path")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON result")
	return cmd
}

func newRollbackCommand() *cobra.Command {
	var profileFlag, rootFlag string
	var yes bool
	cmd := &cobra.Command{
		Use:     "rollback [--profile <id-or-path>] [--root <dir>] [--yes]",
		Short:   "Restore the most recent ConfigHub backup",
		Example: "confighub rollback --profile macbook --root examples --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("rollback does not accept positional arguments")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			profileID, err := resolveProfileID(root, profileFlag)
			if err != nil {
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			res, err := apply.Rollback(apply.RollbackOptions{RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: profileID, Yes: yes})
			if err != nil {
				return &exitError{code: exitRollback, err: err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rolled back bundle %s\n", res.BundleVersion)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile id or YAML path")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive confirmation")
	return cmd
}

func newDoctorCommand() *cobra.Command {
	var profileFlag, rootFlag, bundleFlag string
	cmd := &cobra.Command{Use: "doctor", Short: "Run ConfigHub health checks", Example: "confighub doctor apply --profile macbook --root examples"}
	applyCmd := &cobra.Command{
		Use:   "apply [--profile <id-or-path>] [--root <dir>] [--bundle <path-or-version>]",
		Short: "Check local apply readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("doctor apply does not accept positional arguments")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			profileID, err := resolveProfileID(root, profileFlag)
			if err != nil {
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			bundleDir := ""
			if bundleFlag != "" {
				bundleDir, err = resolveBundleDir(root, profileID, bundleFlag)
				if err != nil {
					return err
				}
			}
			res, err := apply.DoctorApply(context.Background(), apply.DoctorOptions{RootDir: root, StateDir: filepath.Join(root, "state"), HomeDir: home, ProfileID: profileID, BundleDir: bundleDir})
			for _, check := range res.Checks {
				fmt.Fprintf(cmd.OutOrStdout(), "ok: %s\n", check)
			}
			for _, check := range res.Errors {
				fmt.Fprintf(cmd.OutOrStdout(), "fail: %s\n", check)
			}
			if err != nil {
				return &exitError{code: exitValidation, err: err}
			}
			return nil
		},
	}
	applyCmd.Flags().StringVar(&profileFlag, "profile", "", "profile id or YAML path")
	applyCmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	applyCmd.Flags().StringVar(&bundleFlag, "bundle", "", "optional bundle path, version, or latest")
	cmd.AddCommand(applyCmd)
	return cmd
}

func resolveApplyInputs(rootFlag, profileFlag, bundleFlag string, needBundle bool) (string, string, string, string, error) {
	if needBundle && bundleFlag == "" {
		return "", "", "", "", &exitError{code: exitUsage, err: fmt.Errorf("--bundle is required")}
	}
	root, err := filepath.Abs(rootFlag)
	if err != nil {
		return "", "", "", "", &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
	}
	profileID, err := resolveProfileID(root, profileFlag)
	if err != nil {
		return "", "", "", "", err
	}
	bundleDir := ""
	if bundleFlag != "" {
		bundleDir, err = resolveBundleDir(root, profileID, bundleFlag)
		if err != nil {
			return "", "", "", "", err
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", err
	}
	return root, profileID, bundleDir, home, nil
}

func resolveProfileID(root, profileValue string) (string, error) {
	if profileValue != "" {
		if strings.Contains(profileValue, "/") || strings.HasSuffix(profileValue, ".yaml") || strings.HasSuffix(profileValue, ".yml") {
			base := filepath.Base(profileValue)
			return strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml"), nil
		}
		return profileValue, nil
	}
	data, err := os.ReadFile(filepath.Join(root, "state", "active-profile"))
	if err != nil {
		return "", &exitError{code: exitUsage, err: fmt.Errorf("no active profile; pass --profile")}
	}
	profileID := strings.TrimSpace(string(data))
	if profileID == "" {
		return "", &exitError{code: exitUsage, err: fmt.Errorf("no active profile; pass --profile")}
	}
	return profileID, nil
}

func resolveBundleDir(root, profileID, value string) (string, error) {
	if strings.Contains(value, "/") || strings.Contains(value, string(os.PathSeparator)) {
		abs, err := filepath.Abs(value)
		if err != nil {
			return "", &exitError{code: exitUsage, err: fmt.Errorf("resolve --bundle: %w", err)}
		}
		return abs, nil
	}
	version := value
	if value == "latest" {
		latest, err := bundle.LatestBundleVersion(filepath.Join(root, "bundles"), profileID)
		if err != nil {
			return "", err
		}
		if latest == "" {
			return "", &exitError{code: exitUsage, err: fmt.Errorf("no bundles found for profile %q", profileID)}
		}
		version = latest
	}
	return filepath.Join(root, "bundles", profileID, version), nil
}

func mapApplyError(err error) error {
	switch {
	case errors.Is(err, bundle.ErrChecksumMismatch):
		return &exitError{code: exitChecksum, err: err}
	case errors.Is(err, apply.ErrPathPolicy):
		return &exitError{code: exitPathPolicy, err: err}
	case errors.Is(err, apply.ErrLockHeld), errors.Is(err, apply.ErrStaleLock):
		return &exitError{code: exitLock, err: err}
	default:
		return &exitError{code: exitValidation, err: err}
	}
}
