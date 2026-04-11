package cmd

import (
	"fmt"

	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

var secretConfigureCheck bool

func init() {
	secretCmd.AddCommand(secretsListCmd)
	secretCmd.AddCommand(secretGetCmd)
	secretCmd.AddCommand(secretDeleteCmd)
	secretCmd.AddCommand(secretSetCmd)
	secretCmd.AddCommand(secretConfigureCmd)
	secretConfigureCmd.Flags().BoolVar(&secretConfigureCheck, "check", false, "check for missing secrets without prompting")
	rootCmd.AddCommand(secretCmd)
}

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets for the current context",
	Long:  `Manage secrets stored in encrypted storage for the current context.`,
}

var secretSetCmd = &cobra.Command{
	Use:   "set <key>",
	Short: "Set a secret",
	Long:  `Set a secret in the current context. The value is prompted securely and never shown.`,
	Example: `  # Set a new secret (value is prompted securely)
  pilot secret set DB_PASSWORD

  # Update an existing secret
  pilot secret set API_KEY`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectSecretCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleSet(args[0])
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret keys",
	Long:  `List all secret keys for the current context (values are not shown).`,
	Example: `  # List all configured secrets
  pilot secret list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectSecretCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleList()
	},
}

var secretGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get the value of a secret",
	Long:  `Retrieve and display a secret value from the current context's encrypted storage.`,
	Example: `  # Get the value of a secret
  pilot secret get DB_PASSWORD

  # Use in a shell script
  export DB_PASSWORD=$(pilot secret get DB_PASSWORD)`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: SecretKeysCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectSecretCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleGet(args[0])
	},
}

var secretDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a secret",
	Long:  `Remove a secret from the current context's encrypted storage.`,
	Example: `  # Delete a secret
  pilot secret delete DB_PASSWORD`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		secretsRepo, err := app.InjectSecretRepository()
		if err != nil {
			return err
		}
		configRepo, err := app.InjectConfigRepo()
		if err != nil {
			return fmt.Errorf("error injecting config repo: %v", err)
		}
		configContext, err := configRepo.LoadCurrentConfigurationContext()
		if err != nil {
			return fmt.Errorf("error loading current configuration context: %v", err)
		}

		secrets, err := secretsRepo.LoadSecrets(configContext.Name)
		if err != nil {
			return err
		}

		for _, secret := range secrets {
			if secret.Key == args[0] {
				return nil
			}
		}
		return fmt.Errorf("secret '%s' not found", args[0])
	},
	ValidArgsFunction: SecretKeysCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectSecretCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleDelete(args[0])
	},
}

var secretConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure missing secrets interactively",
	Long: `Scan configuration templates for secret references ({{.Secrets.KEY}}) and
prompt for any missing values. Existing secrets are preserved.

Scanned locations:
  - Scripts (scripts section)
  - Helm arguments (services[].helmArgs)
  - Docker build arguments (services[].dockerImages[].buildArgs)

Press Enter to skip a secret during interactive prompts.

Use --check to validate secrets without prompting.`,
	Example: `  # Interactively configure missing secrets
  pilot secret configure

  # Validate secrets without prompting (exits with error if missing)
  pilot secret configure --check`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectSecretCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleConfigure(secretConfigureCheck)
	},
}
