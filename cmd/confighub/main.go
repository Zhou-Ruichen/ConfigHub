package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0-dev"

func main() {
	handleCommandError(newRootCommand().Execute())
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
		newServeCommand(),
		newStubCommand("init", "Configure this client against a ConfigHub hub", "confighub init --from http://127.0.0.1:8787 --profile macbook"),
		newRenderCommand(),
		newStubCommand("pull", "Fetch bundle metadata and files from a hub", "confighub pull --from http://127.0.0.1:8787 --profile macbook --dry-run"),
		newTokenCommand(),
		newStatusCommand(),
		newDiffCommand(),
		newApplyCommand(),
		newRollbackCommand(),
		newDoctorCommand(),
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
