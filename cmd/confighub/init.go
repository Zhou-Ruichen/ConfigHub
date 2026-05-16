package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/pull"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var fromFlag, profileFlag, tokenFlag, rootFlag string
	cmd := &cobra.Command{
		Use:     "init --from <url> --profile <id> --token <token> [--root <dir>]",
		Short:   "Configure this client against a ConfigHub hub",
		Example: "confighub init --from http://127.0.0.1:8787 --profile macbook --token $CONFIGHUB_TOKEN",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &exitError{code: exitUsage, err: fmt.Errorf("init does not accept positional arguments")}
			}
			if fromFlag == "" || profileFlag == "" {
				return &exitError{code: exitUsage, err: fmt.Errorf("--from and --profile are required")}
			}
			if tokenFlag == "" {
				tokenFlag = os.Getenv("CONFIGHUB_TOKEN")
			}
			if tokenFlag == "" {
				return &exitError{code: exitUsage, err: fmt.Errorf("--token is required")}
			}
			hubURL, err := normalizeHubURL(fromFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: err}
			}
			root, err := filepath.Abs(rootFlag)
			if err != nil {
				return &exitError{code: exitUsage, err: fmt.Errorf("resolve --root: %w", err)}
			}
			cfg := &pull.HubConfig{URL: hubURL, Profile: profileFlag, Token: tokenFlag, SchemaVersion: bundle.SupportedSchemaVersion}
			client := pull.New(cfg)
			pub, err := client.FetchSigningKey(context.Background())
			if err != nil {
				return mapPullError(err)
			}
			cfg.PinnedPublicKey = pub
			manifest, err := client.FetchManifest(context.Background())
			if err != nil {
				return mapPullError(err)
			}
			if manifest.SchemaVersion != bundle.SupportedSchemaVersion {
				return mapPullError(pull.ErrSchemaUnsupported)
			}
			if manifest.ProfileID != profileFlag {
				return mapPullError(pull.ErrProfileMismatch)
			}
			if err := cfg.Save(filepath.Join(root, "state")); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "initialized; run `confighub pull` next")
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFlag, "from", "", "hub URL")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile id")
	cmd.Flags().StringVar(&tokenFlag, "token", "", "bearer token (or CONFIGHUB_TOKEN)")
	cmd.Flags().StringVar(&rootFlag, "root", ".", "client state root directory")
	return cmd
}

func normalizeHubURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse --from: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("--from must include scheme and host")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}
