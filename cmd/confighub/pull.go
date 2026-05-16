package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/pull"
	"github.com/ruichen/config-hub/internal/sign"
	"github.com/spf13/cobra"
)

const exitNetwork = 20

func newPullCommand() *cobra.Command {
	var rootFlag string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "pull [--root <dir>] [--dry-run]",
		Short:   "Fetch and verify the latest bundle from the configured hub",
		Example: "confighub pull --root ~/.config/confighub --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("pull does not accept positional arguments")}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			stateDir := filepath.Join(root, "state")
			cfg, err := pull.Load(stateDir)
			if err != nil {
				return &exitError{code: exitUsage, err: err}
			}
			res, err := pull.Pull(context.Background(), cfg, stateDir, pull.PullOptions{DryRun: dryRun})
			if err != nil {
				return mapPullError(err)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would write to %s\n", res.Path)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pulled bundle %s to %s\n", res.BundleVersion, res.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&rootFlag, "root", ".", "client state root directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "fetch and verify without installing into state/pull")
	return cmd
}

func mapPullError(err error) error {
	switch {
	case errors.Is(err, pull.ErrHTTPAuth):
		return &exitError{code: exitAuth, err: err}
	case errors.Is(err, pull.ErrHTTPNotFound):
		return &exitError{code: exitNetwork, err: err}
	case errors.Is(err, pull.ErrManifestUnparseable), errors.Is(err, pull.ErrSchemaUnsupported), errors.Is(err, pull.ErrProfileMismatch):
		return &exitError{code: exitValidation, err: err}
	case errors.Is(err, pull.ErrSignatureAlgorithm), errors.Is(err, pull.ErrPinnedKeyMismatch), errors.Is(err, pull.ErrSignatureInvalid), errors.Is(err, sign.ErrSignatureAlgorithm), errors.Is(err, sign.ErrSignatureInvalid), errors.Is(err, bundle.ErrChecksumMismatch):
		return &exitError{code: exitChecksum, err: err}
	default:
		return &exitError{code: exitNetwork, err: err}
	}
}
