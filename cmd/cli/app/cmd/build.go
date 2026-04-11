package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(buildCmd)
}

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build Docker images for services",
	Long: `Builds Docker images for the specified services. If no services are
specified, builds all services in the current profile.

Images are built using the configured Dockerfile and made available to the
local Kubernetes cluster.`,
	Example: `  # Build all services in the default profile
  pilot build

  # Build specific services
  pilot build api frontend

  # Build all services regardless of profile
  pilot build -p all`,
	Args:              ServiceArgsValidator,
	ValidArgsFunction: ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectBuildCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle(args, *profile)
	},
}
