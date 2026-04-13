package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

const DefaultProfile = "default"

func ServiceArgsValidator(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(home, ".pilot-config.yaml")); err != nil {
			return nil
		}
	}
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return fmt.Errorf("error injecting config repo: %v", err)
	}
	configContext, err := configRepo.LoadCurrentConfigurationContext()
	if err != nil {
		return fmt.Errorf("error loading current configuration context: %v", err)
	}
	for _, service := range args {
		foundService := false
		for _, s := range configContext.Services {
			if service == s.Name {
				foundService = true
				break
			}
		}
		if !foundService {
			return fmt.Errorf("service %s not found", service)
		}
	}

	return nil
}

func ServiceArgsCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	configContext, err := configRepo.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var services []string
	for _, s := range configContext.Services {
		services = append(services, s.Name)
	}

	return services, cobra.ShellCompDirectiveNoFileComp
}

func SecretKeysCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	secretsRepo, err := app.InjectSecretRepository()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	currentContextName, err := configRepo.LoadCurrentContextName()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	secrets, err := secretsRepo.LoadSecrets(currentContextName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var secretKeys []string
	for _, secret := range secrets {
		secretKeys = append(secretKeys, secret.Key)
	}
	return secretKeys, cobra.ShellCompDirectiveNoFileComp
}
