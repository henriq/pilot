package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the application version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Pilot %s\n", version)
	},
}
