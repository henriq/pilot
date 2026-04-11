package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

var pullSkipConfirmation bool

func init() {
	rootCmd.AddCommand(pullCmd)
	pullCmd.Flags().BoolVarP(&pullSkipConfirmation, "yes", "y", false, "skip confirmation for overwriting locally-built images")
}

var pullCmd = &cobra.Command{
	Use:   "pull [service...]",
	Short: "Pull Docker images for services",
	Long: `Pulls Docker images for the specified services. If no services are
specified, pulls all images for services in the current profile.

This command pulls both remote images and locally-built images. When pulling
locally-built images, you will be prompted for confirmation since this will
overwrite any locally built versions. Use --yes to skip confirmation.`,
	Example: `  # Pull all images in the default profile
  pilot pull

  # Pull images for specific services
  pilot pull api frontend

  # Pull all images regardless of profile
  pilot pull -p all

  # Skip confirmation for overwriting locally-built images
  pilot pull --yes`,
	Args:              ServiceArgsValidator,
	ValidArgsFunction: ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectPullCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle(args, *profile, pullSkipConfirmation)
	},
}
