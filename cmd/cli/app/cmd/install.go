package cmd

import (
	"pilot/cmd/cli/app"
	"pilot/cmd/cli/app/cmd/arguments"

	"github.com/spf13/cobra"
)

var installHelmChartOverrides []string

func init() {
	installCmd.Flags().BoolP("intercept-http", "i", false, "Enable HTTP interception via mitmweb proxy")
	arguments.RegisterHelmChartOverrideFlag(installCmd, &installHelmChartOverrides)
	rootCmd.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "install [service...]",
	Short: "Deploy services to Kubernetes",
	Long: `Deploys the specified services to the local Kubernetes cluster using Helm.
If no services are specified, deploys all services in the current profile.

This provisions TLS certificates, sets up the dev-proxy for local service
routing, and deploys each service via Helm.

Use --intercept-http to enable HTTP traffic interception via mitmweb. When
enabled, the mitmweb UI is available at:

  https://dev-proxy.<context>.localhost`,
	Example: `  # Install all services in the default profile
  pilot install

  # Install specific services
  pilot install api database

  # Install with HTTP traffic interception
  pilot install --intercept-http

  # Install all services regardless of profile
  pilot install -p all

  # Render a service's Helm chart from a local directory
  pilot install api --helm-chart api:./charts/api`,
	Args:              arguments.ServiceArgsValidator,
	ValidArgsFunction: arguments.ServiceArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		interceptHttp, _ := cmd.Flags().GetBool("intercept-http")
		helmChartOverrides, err := arguments.ParseHelmChartOverrides(installHelmChartOverrides)
		if err != nil {
			return err
		}
		handler, err := app.InjectInstallCommandHandler()
		if err != nil {
			return err
		}

		return handler.Handle(args, *profile, interceptHttp, helmChartOverrides)
	},
}
