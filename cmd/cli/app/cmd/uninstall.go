package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall [service...]",
	Short: "Remove services from Kubernetes",
	Long: `Removes the specified services from the local Kubernetes cluster.
If no services are specified, removes all services in the current profile.

This uses Helm to uninstall the deployed releases.`,
	Example: `  # Uninstall all services in the default profile
  pilot uninstall

  # Uninstall specific services
  pilot uninstall api database

  # Uninstall all services regardless of profile
  pilot uninstall -p all`,
	Args:              ServiceArgsValidator,
	ValidArgsFunction: ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectUninstallCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle(args, *profile)
	},
}
