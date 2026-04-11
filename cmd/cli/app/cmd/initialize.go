package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initializeCmd)
}

var initializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Create a sample configuration file",
	Long: `Create a new configuration file at ~/.pilot-config.yaml with sample values
for all configuration options. The file is not created if it already exists.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectInitializeCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle()
	},
}
