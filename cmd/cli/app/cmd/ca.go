package cmd

import (
	"fmt"
	"os"
	"strings"

	"dx/cmd/cli/app"
	"dx/internal/cli/output"

	"github.com/spf13/cobra"
)

var (
	caDeleteSkipConfirmation bool
	caIssueOut               string
	caIssueKeyOut            string
	caIssueCAOut             string
	caIssueType              string
)

func init() {
	caCmd.AddCommand(caPrintCmd)
	caCmd.AddCommand(caDeleteCmd)
	caCmd.AddCommand(caStatusCmd)
	caCmd.AddCommand(caIssueCmd)
	caDeleteCmd.Flags().BoolVarP(&caDeleteSkipConfirmation, "yes", "y", false, "skip confirmation prompt")
	caIssueCmd.Flags().StringVar(&caIssueOut, "out", "", "path to write the certificate PEM file")
	caIssueCmd.Flags().StringVar(&caIssueKeyOut, "keyout", "", "path to write the private key PEM file")
	caIssueCmd.Flags().StringVar(&caIssueCAOut, "caout", "", "path to write the CA certificate PEM file")
	caIssueCmd.Flags().StringVar(&caIssueType, "type", "", "certificate type (server, client)")
	_ = caIssueCmd.MarkFlagRequired("out")
	_ = caIssueCmd.MarkFlagRequired("keyout")
	_ = caIssueCmd.MarkFlagRequired("type")
	_ = caIssueCmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"server", "client"}, cobra.ShellCompDirectiveNoFileComp
	})
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

var caIssueCmd = &cobra.Command{
	Use:   "issue <dns-names...>",
	Args:  cobra.MinimumNArgs(1),
	Short: "Issue a certificate from the context's private CA",
	Long: `Issue a new certificate signed by the context's private CA and write the
certificate and private key to local files. The CA is loaded (or created if
none exists) automatically.

DNS names must use reserved TLDs only (.localhost, .test, .example, .invalid,
.local, .internal, .home.arpa).`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Example: `  # Issue a server certificate
  dx ca issue myapp.test --type server --out cert.pem --keyout key.pem

  # Issue a client certificate with multiple SANs
  dx ca issue api.test *.api.test --type client --out client.pem --keyout client-key.pem

  # Also save the CA certificate
  dx ca issue myapp.localhost --type server --out cert.pem --keyout key.pem --caout ca.pem`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectCACommandHandler()
		if err != nil {
			return err
		}

		contextName, issued, err := handler.HandleIssue(caIssueType, args)
		if err != nil {
			return err
		}

		if err := os.WriteFile(caIssueOut, issued.CertPEM, 0600); err != nil {
			return fmt.Errorf("failed to write certificate to '%s': %w", caIssueOut, err)
		}

		if err := os.WriteFile(caIssueKeyOut, issued.KeyPEM, 0600); err != nil {
			return fmt.Errorf("failed to write private key to '%s': %w", caIssueKeyOut, err)
		}

		if caIssueCAOut != "" {
			if err := os.WriteFile(caIssueCAOut, issued.CAPEM, 0600); err != nil {
				return fmt.Errorf("failed to write CA certificate to '%s': %w", caIssueCAOut, err)
			}
		}

		output.PrintSuccess("Issued certificate for context '" + contextName + "'")
		output.PrintField("Certificate:", caIssueOut)
		output.PrintField("Private key:", caIssueKeyOut)
		if caIssueCAOut != "" {
			output.PrintField("CA cert:", caIssueCAOut)
		}
		output.PrintField("Type:", caIssueType)
		output.PrintField("DNS names:", strings.Join(args, ", "))

		return nil
	},
}
