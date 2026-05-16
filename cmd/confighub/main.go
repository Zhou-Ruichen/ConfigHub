package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0-dev"

func main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "confighub",
		Short:         "Manage rendered configuration bundles",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	cmd.AddCommand(
		newStubCommand("serve", "Run the ConfigHub web UI and HTTP API", "confighub serve --bind 127.0.0.1:8787 --root examples"),
		newStubCommand("init", "Configure this client against a ConfigHub hub", "confighub init --from http://127.0.0.1:8787 --profile macbook"),
		newStubCommand("render", "Render templates into an immutable bundle", "confighub render --profile macbook"),
		newStubCommand("pull", "Fetch bundle metadata and files from a hub", "confighub pull --from http://127.0.0.1:8787 --profile macbook --dry-run"),
		newStubCommand("status", "Show local ConfigHub state", "confighub status --profile macbook"),
		newStubCommand("diff", "Compare a bundle with local targets", "confighub diff --bundle examples/bundles/macbook/2026-05-16T15-04-22Z-001"),
		newStubCommand("apply", "Back up and apply a rendered bundle", "confighub apply --bundle examples/bundles/macbook/2026-05-16T15-04-22Z-001 --dry-run"),
		newStubCommand("rollback", "Restore the most recent ConfigHub backup", "confighub rollback --profile macbook --dry-run"),
		newStubCommand("doctor", "Run ConfigHub health checks", "confighub doctor --profile macbook"),
	)

	return cmd
}

func newStubCommand(use, short, example string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: example,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.ErrOrStderr(), "not implemented")
			os.Exit(1)
		},
	}
}
