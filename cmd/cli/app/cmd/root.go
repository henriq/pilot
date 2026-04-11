package cmd

import (
	"os"
	"path/filepath"

	"pilot/cmd/cli/app"
	"pilot/internal/cli/output"

	"github.com/spf13/cobra"
)

var profile *string

var rootCmd = &cobra.Command{
	Use:   "pilot",
	Short: "Automate local development for Kubernetes-hosted services",
	Long: `Pilot automates builds, deployments, and local development workflows for
services running in Kubernetes. Define your services in a YAML configuration
file and Pilot handles the rest: Docker builds, Helm deployments, traffic
routing, TLS certificates, encrypted secrets, and HTTP interception.

Configuration is stored in ~/.pilot-config.yaml. Run 'pilot initialize' to create
a sample configuration file.

Common workflows:
  pilot update                   Build images and deploy services
  pilot install                  Deploy services to Kubernetes
  pilot build                    Build Docker images
  pilot context set <name>       Switch to a different context
  pilot context info             Show services, URLs, and status`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if home, err := os.UserHomeDir(); err == nil {
			if _, err := os.Stat(filepath.Join(home, ".pilot-config.yaml")); err == nil {
				return nil
			}
		}
		handler, err := app.InjectMigrateCommandHandler()
		if err != nil {
			return err
		}
		_, err = handler.Handle()
		return err
	},
}

func Execute() {
	profile = rootCmd.PersistentFlags().StringP("profile", "p", DefaultProfile, "Profile to use")
	if err := rootCmd.Execute(); err != nil {
		output.PrintError(err.Error())
		os.Exit(1)
	}
}
