package main

import "github.com/spf13/cobra"

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
		newInitCommand(),
		newRenderCommand(),
		newPullCommand(),
		newTokenCommand(),
		newStatusCommand(),
		newDiffCommand(),
		newApplyCommand(),
		newRollbackCommand(),
		newDoctorCommand(),
	)

	return cmd
}
