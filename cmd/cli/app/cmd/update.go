package cmd

import (
	"pilot/cmd/cli/app"
	"pilot/cmd/cli/app/cmd/arguments"
	"pilot/internal/cli/output"

	"github.com/spf13/cobra"
)

var pullImages bool
var updateHelmChartOverrides []string
var updateImageSourceOverrides []string
var updateServiceGitRefOverrides []string

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&pullImages, "pull", false, "pull images instead of building them")
	updateCmd.Flags().BoolP("intercept-http", "i", false, "Enable HTTP interception via mitmweb proxy")
	arguments.RegisterHelmChartOverrideFlag(updateCmd, &updateHelmChartOverrides)
	arguments.RegisterImageSourceOverrideFlag(updateCmd, &updateImageSourceOverrides)
	arguments.RegisterServiceGitRefOverrideFlag(updateCmd, &updateServiceGitRefOverrides)
}

var updateCmd = &cobra.Command{
	Use:   "update [service...]",
	Short: "Build and redeploy services",
	Long: `Builds (or pulls with --pull) and redeploys the selected services.
If no services are specified, updates all services in the current profile.

This is the most common command during development — it rebuilds images and
redeploys services in one step.

Use --pull to pull pre-built images from the registry instead of building.
Unlike 'pilot pull', this skips the confirmation prompt since --pull is an
explicit opt-in to overwrite locally-built images.

Use --intercept-http to enable HTTP traffic interception via mitmweb.

Use --git-ref to clone the configured git repo at a different ref than the one
in ~/.pilot-config.yaml — handy for testing feature branches.

Use --image-source to build an image from a local directory instead of cloning
its configured git repo, and --helm-chart to render a service's chart from a
local directory instead of cloning it.`,
	Example: `  # Build and redeploy all services in the default profile
  pilot update

  # Update specific services
  pilot update api frontend

  # Pull images instead of building, then redeploy
  pilot update --pull

  # Pull and redeploy specific services
  pilot update --pull api frontend

  # Build and redeploy with HTTP traffic interception
  pilot update --intercept-http

  # Render a service's Helm chart from a local directory
  pilot update api --helm-chart api:./charts/api

  # Build an image from a local checkout, then redeploy
  pilot update api --image-source api:./services/api

  # Build from a feature branch, then redeploy
  pilot update api --git-ref api:feature/my-change`,
	Args:              arguments.ServiceArgsValidator,
	ValidArgsFunction: arguments.ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		interceptHttp, _ := cmd.Flags().GetBool("intercept-http")
		helmChartOverrides, err := arguments.ParseHelmChartOverrides(updateHelmChartOverrides)
		if err != nil {
			return err
		}
		imageSourceOverrides, err := arguments.ParseImageSourceOverrides(updateImageSourceOverrides)
		if err != nil {
			return err
		}
		serviceGitRefOverrides, err := arguments.ParseServiceGitRefOverrides(updateServiceGitRefOverrides)
		if err != nil {
			return err
		}
		if pullImages {
			if !imageSourceOverrides.IsEmpty() {
				output.PrintWarning("--image-source has no effect when --pull is set; ignoring")
			}
			if !serviceGitRefOverrides.IsEmpty() {
				output.PrintWarning("--git-ref has no effect when --pull is set; ignoring")
			}
			pullHandler, err := app.InjectPullCommandHandler()
			if err != nil {
				return err
			}
			// skipConfirmation=true since update is an intentional action
			err = pullHandler.Handle(args, *profile, true)
			if err != nil {
				return err
			}
		} else {
			buildHandler, err := app.InjectBuildCommandHandler()
			if err != nil {
				return err
			}
			err = buildHandler.Handle(args, *profile, imageSourceOverrides, serviceGitRefOverrides)
			if err != nil {
				return err
			}
		}

		installHandler, err := app.InjectInstallCommandHandler()
		if err != nil {
			return err
		}

		return installHandler.Handle(args, *profile, interceptHttp, helmChartOverrides)
	},
}
