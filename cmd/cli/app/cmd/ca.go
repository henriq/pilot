package cmd

import (
	"dx/cmd/cli/app"

	"github.com/spf13/cobra"
)

var caRecreateSkipConfirmation bool

func init() {
	caCmd.AddCommand(caPrintCmd)
	caCmd.AddCommand(caReissueCmd)
	caCmd.AddCommand(caRecreateCmd)
	caCmd.AddCommand(caStatusCmd)
	caRecreateCmd.Flags().BoolVarP(&caRecreateSkipConfirmation, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(caCmd)
}

var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Manage the certificate authority for the current context",
	Long: `Manage the private certificate authority (CA) used to issue TLS certificates
for services in the current context. The CA is created automatically during
the first 'dx install' when certificates are configured.`,
	Example: `  # Check current CA status
  dx ca status

  # Print the CA certificate for trust store setup
  dx ca print > ca.crt

  # Re-issue all certificates (non-destructive)
  dx ca reissue

  # Delete and recreate the CA (destructive)
  dx ca recreate`,
}

var caPrintCmd = &cobra.Command{
	Use:   "print",
	Args:  cobra.NoArgs,
	Short: "Print the CA certificate in PEM format",
	Long: `Print the PEM-encoded CA certificate to stdout. The output contains only
the certificate with no extra decoration, making it safe for piping.

Use this to add the CA to your system trust store or browser for local development.`,
	Example: `  # Print the CA certificate
  dx ca print

  # Save to a file
  dx ca print > ca.crt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCACommandHandler()
		if err != nil {
			return err
		}
		return handler.HandlePrint()
	},
}

var caReissueCmd = &cobra.Command{
	Use:   "reissue",
	Args:  cobra.NoArgs,
	Short: "Re-issue all certificates using the existing CA",
	Long: `Generate new certificates for all configured services using the existing CA.
The CA certificate itself is not changed.

Existing Kubernetes secrets are updated with the new certificates. Run
'dx install' afterwards to restart services with the new certificates.`,
	Example: `  # Re-issue all certificates
  dx ca reissue

  # Re-issue and apply
  dx ca reissue && dx install`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCACommandHandler()
		if err != nil {
			return err
		}
		return handler.HandleReissue()
	},
}

var caRecreateCmd = &cobra.Command{
	Use:   "recreate",
	Args:  cobra.NoArgs,
	Short: "Delete and recreate the CA and all certificates",
	Long: `Delete the existing CA, create a new one, and re-issue all configured
certificates. This is a destructive operation that invalidates all previously
issued certificates.

After recreating, you must:
  1. Run 'dx install' to apply the new certificates
  2. Update your system trust store with the new CA certificate`,
	Example: `  # Recreate with confirmation prompt
  dx ca recreate

  # Skip confirmation (for scripting)
  dx ca recreate --yes

  # Full workflow
  dx ca recreate && dx install`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCACommandHandler()
		if err != nil {
			return err
		}
		return handler.HandleRecreate(caRecreateSkipConfirmation)
	},
}

var caStatusCmd = &cobra.Command{
	Use:   "status",
	Args:  cobra.NoArgs,
	Short: "Show the status of the certificate authority",
	Long: `Show the current state of the certificate authority including its validity
period and expiration date. Also lists all configured certificates with their
Kubernetes secret name, type, DNS names, and provisioning status.`,
	Example: `  # Check CA status
  dx ca status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCACommandHandler()
		if err != nil {
			return err
		}
		return handler.HandleStatus()
	},
}
