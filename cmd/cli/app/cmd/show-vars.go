package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showVarsCommand)
}

var showVarsCommand = &cobra.Command{
	Use:   "show-vars",
	Short: "Show variables available in configuration templates",
	Long:  `Show all variables that can be referenced in configuration templates (e.g. secrets, service properties).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectShowVarsCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle()
	},
}
