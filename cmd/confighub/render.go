package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/render"
	"github.com/spf13/cobra"
)

const (
	exitUsage      = 2
	exitValidation = 10
	exitLock       = 13
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func newRenderCommand() *cobra.Command {
	var profileFlag string
	var rootFlag string
	var dryRun bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "render --profile <id-or-path> [--root <dir>] [--dry-run] [--json]",
		Short: "Render templates into an immutable bundle",
		Example: "confighub render --profile macbook --root examples\n" +
			"confighub render --profile examples/profiles/macbook.yaml --root . --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("render does not accept positional arguments")}
			}
			if profileFlag == "" {
				return &exitError{code: exitUsage, err: fmt.Errorf("--profile is required")}
			}

			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			profilePath := resolveProfilePath(root, profileFlag)

			release, err := render.AcquireRenderLock(filepath.Join(root, "state"))
			if err != nil {
				if errors.Is(err, render.ErrLockHeld) || errors.Is(err, render.ErrStaleLock) {
					return &exitError{code: exitLock, err: err}
				}
				return err
			}
			defer release()

			result, err := render.Render(context.Background(), render.Options{
				ProfilePath: profilePath,
				RootDir:     root,
				DryRun:      dryRun,
				JSON:        jsonOut,
			})
			if err != nil {
				return &exitError{code: exitValidation, err: err}
			}

			if !dryRun {
				if err := bundle.WriteAtomic(filepath.Join(root, "bundles"), result.Manifest.ProfileID, result.Manifest.BundleVersion, result.Files, result.Manifest, result.Checksums); err != nil {
					return err
				}
			}

			if dryRun || jsonOut {
				data, err := json.MarshalIndent(result.Manifest, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			bundleDir := filepath.Join(root, "bundles", result.Manifest.ProfileID, result.Manifest.BundleVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "rendered bundle %s\n", bundleDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile id or YAML path")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "hub root directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "render to memory and write nothing")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print manifest JSON on success")
	return cmd
}

func resolveProfilePath(root, profileValue string) string {
	if strings.Contains(profileValue, "/") || strings.HasSuffix(profileValue, ".yaml") || strings.HasSuffix(profileValue, ".yml") {
		return profileValue
	}
	return filepath.Join(root, "profiles", profileValue+".yaml")
}

func handleCommandError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	var exitErr *exitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.code)
	}
	os.Exit(1)
}
