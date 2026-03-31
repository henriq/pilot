package cmd

import (
	"dx/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	installCmd.Flags().BoolP("intercept-http", "i", false, "Enable HTTP interception via mitmweb proxy")
	rootCmd.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "install [service...]",
	Short: "Deploy services to Kubernetes via Helm",
	Long: `Deploys the specified services to the local Kubernetes cluster using Helm.
If no services are specified, deploys all services in the current profile.

This command also sets up the dev-proxy (HAProxy) for routing traffic between
local and Kubernetes services.

Use --intercept-http to deploy mitmweb alongside HAProxy for inspecting and
replaying HTTP traffic. When enabled, the mitmweb UI is available at:

  http://dev-proxy.<context>.localhost

Without --intercept-http, the dev-proxy routes traffic directly through HAProxy
without HTTP-level inspection.`,
	Example: `  # Install all services in the default profile
  dx install

  # Install specific services
  dx install api database

  # Install with HTTP traffic interception
  dx install --intercept-http

  # Install all services regardless of profile
  dx install -p all`,
	Args:              ServiceArgsValidator,
	ValidArgsFunction: ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		interceptHttp, _ := cmd.Flags().GetBool("intercept-http")
		handler, err := app.InjectInstallCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle(args, *profile, interceptHttp)
	},
}
