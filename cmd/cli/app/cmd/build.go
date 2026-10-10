package cmd

import (
	"pilot/cmd/cli/app"
	"pilot/cmd/cli/app/cmd/arguments"

	"github.com/spf13/cobra"
)

var buildImageSourceOverrides []string
var buildServiceGitRefOverrides []string

func init() {
	rootCmd.AddCommand(buildCmd)
	arguments.RegisterImageSourceOverrideFlag(buildCmd, &buildImageSourceOverrides)
	arguments.RegisterServiceGitRefOverrideFlag(buildCmd, &buildServiceGitRefOverrides)
}

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build Docker images for services",
	Long: `Builds Docker images for the specified services. If no services are
specified, builds all services in the current profile.

Images are built using the configured Dockerfile and made available to the
local Kubernetes cluster.

Use --image-source to build an image from a local directory instead of
cloning its configured git repo — handy for testing local changes before
committing or pushing.

Use --git-ref to clone the configured git repo at a different ref than the
one in ~/.pilot-config.yaml — handy for testing feature branches.`,
	Example: `  # Build all services in the default profile
  pilot build

  # Build specific services
  pilot build api frontend

  # Build all services regardless of profile
  pilot build -p all

  # Build an image from a local checkout
  pilot build api --image-source api:./services/api

  # Build from a feature branch
  pilot build api --git-ref api:feature/my-change`,
	Args:              arguments.ServiceArgsValidator,
	ValidArgsFunction: arguments.ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		imageSourceOverrides, err := arguments.ParseImageSourceOverrides(buildImageSourceOverrides)
		if err != nil {
			return err
		}
		serviceGitRefOverrides, err := arguments.ParseServiceGitRefOverrides(buildServiceGitRefOverrides)
		if err != nil {
			return err
		}
		handler, err := app.InjectBuildCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle(args, *profile, imageSourceOverrides, serviceGitRefOverrides)
	},
}
