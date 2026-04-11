package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(genEnvKeyCmd)
}

var genEnvKeyCmd = &cobra.Command{
	Use:   "gen-env-key",
	Short: "Generate an environment key for the active cluster",
	Long:  `Generate an environment key derived from the current cluster and namespace in ~/.kube/config. This key is used for cluster verification.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectGenEnvKeyCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle()
	},
}
