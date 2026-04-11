package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

var pullImages bool

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&pullImages, "pull", false, "pull images instead of building them")
	updateCmd.Flags().BoolP("intercept-http", "i", false, "Enable HTTP interception via mitmweb proxy")
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

Use --intercept-http to enable HTTP traffic interception via mitmweb.`,
	Example: `  # Build and redeploy all services in the default profile
  pilot update

  # Update specific services
  pilot update api frontend

  # Pull images instead of building, then redeploy
  pilot update --pull

  # Pull and redeploy specific services
  pilot update --pull api frontend

  # Build and redeploy with HTTP traffic interception
  pilot update --intercept-http`,
	Args:              ServiceArgsValidator,
	ValidArgsFunction: ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		interceptHttp, _ := cmd.Flags().GetBool("intercept-http")
		if pullImages {
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
			err = buildHandler.Handle(args, *profile)
			if err != nil {
				return err
			}
		}

		installHandler, err := app.InjectInstallCommandHandler()
		if err != nil {
			return err
		}

		return installHandler.Handle(args, *profile, interceptHttp)
	},
}
