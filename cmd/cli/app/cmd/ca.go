package cmd

import (
	"dx/cmd/cli/app"

	"github.com/spf13/cobra"
)

var caDeleteSkipConfirmation bool

func init() {
	caCmd.AddCommand(caPrintCmd)
	caCmd.AddCommand(caDeleteCmd)
	caCmd.AddCommand(caStatusCmd)
	caDeleteCmd.Flags().BoolVarP(&caDeleteSkipConfirmation, "yes", "y", false, "skip confirmation prompt")
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

  # Delete the local CA files
  dx ca delete`,
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

var caDeleteCmd = &cobra.Command{
	Use:   "delete",
	Args:  cobra.NoArgs,
	Short: "Delete the local CA for the current context",
	Long: `Delete the local CA files and its passphrase from the keyring for the current
context. A new CA will be created automatically on the next 'dx install',
which will also issue fresh certificates signed by the new CA.

After installing, update your system trust store with the new CA certificate.`,
	Example: `  # Delete the local CA files
  dx ca delete

  # Skip confirmation (for scripting)
  dx ca delete --yes

  # Delete and recreate the CA
  dx ca delete && dx install`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCACommandHandler()
		if err != nil {
			return err
		}
		return handler.HandleDelete(caDeleteSkipConfirmation)
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
