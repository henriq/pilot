package cmd

import (
	"os"
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	generateCmd.AddCommand(generateHostEntriesCmd)
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate data from configuration",
	Long:  `Generate data derived from the current configuration.`,
	Example: `  # Generate host file entries
  pilot generate host-entries`,
}

var generateHostEntriesCmd = &cobra.Command{
	Use:   "host-entries",
	Short: "Generate /etc/hosts entries for the current context",
	Long:  `Generate host entries for the current context in a format that can be appended to /etc/hosts.`,
	Example: `  # Print host entries
  pilot generate host-entries

  # Append to /etc/hosts
  pilot generate host-entries | sudo tee -a /etc/hosts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectGenerateCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleGenerateHostEntries(os.Stdout)
	},
}
