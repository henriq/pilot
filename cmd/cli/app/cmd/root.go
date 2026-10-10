package cmd

import (
	"os"
	"path/filepath"

	"pilot/cmd/cli/app"
	"pilot/internal/cli/output"

	"github.com/spf13/cobra"
)

const DefaultProfile = "default"

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
		if !skipMigration(cmd) {
			if home, err := os.UserHomeDir(); err == nil {
				if _, err := os.Stat(filepath.Join(home, ".pilot-config.yaml")); err != nil {
					handler, err := app.InjectMigrateCommandHandler()
					if err != nil {
						return err
					}
					if _, err = handler.Handle(); err != nil {
						return err
					}
				}
			}
		}
		if !skipVersionCheck(cmd) {
			warnIfVersionTooOld()
		}
		return nil
	},
}

var migrationSkipCommands = map[string]struct{}{
	"completion": {}, // called by homebrew during link/install where stdin is a TTY, causing the migration prompt to hang
}

func skipMigration(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if _, ok := migrationSkipCommands[c.Name()]; ok {
			return true
		}
	}
	return false
}

var versionCheckSkipCommands = map[string]struct{}{
	"completion": {}, // shell-eval'd by package managers; stderr noise pollutes new shells
	"version":    {}, // already prints the running version, so a "you're old" line above it is redundant
}

func skipVersionCheck(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if _, ok := versionCheckSkipCommands[c.Name()]; ok {
			return true
		}
	}
	return false
}

func warnIfVersionTooOld() {
	versionCheck, err := app.InjectVersionCheckCommandHandler()
	if err != nil {
		return
	}
	versionCheck.Handle(version)
}

func Execute() {
	profile = rootCmd.PersistentFlags().StringP("profile", "p", DefaultProfile, "Profile to use")
	if err := rootCmd.Execute(); err != nil {
		output.PrintError(err.Error())
		os.Exit(1)
	}
}
