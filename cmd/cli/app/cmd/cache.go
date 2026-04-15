package cmd

import (
	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

var cacheClearSkipConfirmation bool
var cacheStatusAll bool
var cacheClearAll bool

func init() {
	cacheCmd.AddCommand(cacheStatusCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheStatusCmd.Flags().BoolVar(&cacheStatusAll, "all", false, "show cache for all contexts")
	cacheClearCmd.Flags().BoolVarP(&cacheClearSkipConfirmation, "yes", "y", false, "skip confirmation prompt")
	cacheClearCmd.Flags().BoolVar(&cacheClearAll, "all", false, "clear cache for all contexts")
	rootCmd.AddCommand(cacheCmd)
}

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage cached data",
	Long: `Manage cached data stored locally for the current context. Cached data
includes cloned git repositories, generated wrapper charts, and dev-proxy
build artifacts. State such as the certificate authority, secrets, and
environment keys is not affected by cache operations.`,
	Example: `  # Show cache size for the current context
  pilot cache status

  # Clear cached data for the current context
  pilot cache clear

  # Clear without confirmation (for scripting)
  pilot cache clear --yes

  # Clear cached data for all contexts
  pilot cache clear --all`,
}

var cacheStatusCmd = &cobra.Command{
	Use:               "status",
	Args:              cobra.NoArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	Short:             "Show cache size",
	Long: `Show the total size of cached data for the current context, with a
breakdown by category. Cached data includes cloned chart and service
repositories, generated wrapper charts, and dev-proxy build artifacts.

Use --all to show cache across all configured contexts.`,
	Example: `  # Show cache size
  pilot cache status

  # Show cache size for all contexts
  pilot cache status --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCacheCommandHandler()
		if err != nil {
			return err
		}
		return handler.HandleStatus(cacheStatusAll)
	},
}

var cacheClearCmd = &cobra.Command{
	Use:               "clear",
	Args:              cobra.NoArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	Short:             "Remove cached data",
	Long: `Remove all cached data for the current context. This includes cloned
git repositories, generated wrapper charts, and dev-proxy build artifacts.
State such as the certificate authority, secrets, and environment keys is
preserved. Use --all to clear cache for all configured contexts.

Cleared data is re-downloaded or regenerated automatically on the next
'pilot install' or 'pilot build'.`,
	Example: `  # Clear cache for the current context
  pilot cache clear

  # Skip confirmation (for scripting)
  pilot cache clear --yes

  # Clear cache for all contexts
  pilot cache clear --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCacheCommandHandler()
		if err != nil {
			return err
		}
		return handler.HandleClear(cacheClearSkipConfirmation, cacheClearAll)
	},
}
