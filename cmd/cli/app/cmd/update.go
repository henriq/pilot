package cmd

import (
	"dx/cmd/cli/app"

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
	Short: "Update services by building or pulling images and reinstalling",
	Long: `Builds (or pulls with --pull) and reinstalls the selected services.
If no services are specified, updates all services in the current profile.

By default, images are built from source. Use --pull to pull pre-built images
from the registry instead. Unlike 'dx pull', this skips the confirmation
prompt since --pull is an explicit opt-in to overwrite locally-built images.

Use --intercept-http to deploy mitmweb alongside HAProxy for inspecting and
replaying HTTP traffic. When enabled, the mitmweb UI is available at:

  https://dev-proxy.<context>.localhost`,
	Example: `  # Build and reinstall all services in the default profile
  dx update

  # Update specific services
  dx update api frontend

  # Pull images instead of building, then reinstall
  dx update --pull

  # Pull and reinstall specific services
  dx update --pull api frontend

  # Build and reinstall with HTTP traffic interception
  dx update --intercept-http`,
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
