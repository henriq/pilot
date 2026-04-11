package handler

import (
	"fmt"
	"sort"
	"strings"

	"pilot/internal/cli/output"
	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

type SecretCommandHandler struct {
	secretsRepository ports.SecretsRepository
	configRepository  ports.ConfigRepository
	terminalInput     ports.TerminalInput
}

func NewSecretCommandHandler(
	secretsRepository ports.SecretsRepository,
	configRepository ports.ConfigRepository,
	terminalInput ports.TerminalInput,
) SecretCommandHandler {
	return SecretCommandHandler{
		secretsRepository: secretsRepository,
		configRepository:  configRepository,
		terminalInput:     terminalInput,
	}
}

func (h *SecretCommandHandler) HandleSet(key string) error {
	if !h.terminalInput.IsTerminal() {
		return fmt.Errorf("cannot read secret value: no terminal available")
	}

	prompt := fmt.Sprintf("Enter value for %s: ", output.Bold(key))
	value, err := h.terminalInput.ReadPassword(prompt)
	if err != nil {
		return fmt.Errorf("failed to read secret value: %w", err)
	}

	if value == "" {
		return fmt.Errorf("secret value cannot be empty")
	}

	configContextName, err := h.configRepository.LoadCurrentContextName()
	if err != nil {
		return err
	}
	secrets, err := h.secretsRepository.LoadSecrets(configContextName)
	if err != nil {
		return err
	}

	var secretExists bool
	for i := range secrets {
		if secrets[i].Key == key {
			secrets[i].Value = value
			secretExists = true
		}
	}

	if !secretExists {
		if conflicting, found := findConflictingSecretKey(secrets, key); found {
			return fmt.Errorf("cannot set secret '%s': conflicts with existing secret '%s' (a secret key cannot have both a direct value and nested keys); delete '%s' first with 'pilot secret delete %s'", key, conflicting, conflicting, conflicting)
		}
		secrets = append(secrets, &domain.Secret{Key: key, Value: value})
	}

	err = h.secretsRepository.SaveSecrets(secrets, configContextName)
	if err != nil {
		return err
	}
	output.PrintSuccess(fmt.Sprintf("Secret '%s' saved", key))
	return nil
}

func (h *SecretCommandHandler) HandleList() error {
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	secrets, err := h.secretsRepository.LoadSecrets(configContext.Name)
	if err != nil {
		return err
	}

	if len(secrets) == 0 {
		output.PrintInfo("No secrets configured")
		return nil
	}

	output.PrintHeader("Secrets")
	output.PrintNewline()

	// Sort secrets by key
	sort.Slice(
		secrets, func(i, j int) bool {
			return secrets[i].Key < secrets[j].Key
		},
	)
	for _, secret := range secrets {
		output.PrintBullet(output.Bold(secret.Key))
	}

	return nil
}

func (h *SecretCommandHandler) HandleGet(key string) error {
	configContextName, err := h.configRepository.LoadCurrentContextName()
	if err != nil {
		return err
	}
	secrets, err := h.secretsRepository.LoadSecrets(configContextName)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if secret.Key == key {
			fmt.Println(secret.Value)
			return nil
		}
	}

	return fmt.Errorf("secret '%s' not found", key)
}

func (h *SecretCommandHandler) HandleDelete(key string) error {
	configContextName, err := h.configRepository.LoadCurrentContextName()
	if err != nil {
		return err
	}
	secrets, err := h.secretsRepository.LoadSecrets(configContextName)
	if err != nil {
		return err
	}
	var newSecrets []*domain.Secret
	for _, secret := range secrets {
		if secret.Key != key {
			newSecrets = append(newSecrets, secret)
		}
	}
	err = h.secretsRepository.SaveSecrets(newSecrets, configContextName)
	if err != nil {
		return err
	}
	output.PrintSuccess(fmt.Sprintf("Secret '%s' deleted", key))
	return nil
}

// HandleConfigure discovers expected secrets from templates and prompts for missing values.
// If checkOnly is true, it only validates and reports missing secrets without prompting.
func (h *SecretCommandHandler) HandleConfigure(checkOnly bool) error {
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	// Extract expected secret keys from templates
	expectedKeys := core.ExtractSecretKeys(configContext)
	if len(expectedKeys) == 0 {
		output.PrintInfo("No secrets referenced in configuration templates")
		return nil
	}

	// Load existing secrets
	existingSecrets, err := h.secretsRepository.LoadSecrets(configContext.Name)
	if err != nil {
		return err
	}

	// Build set of existing keys
	existingKeys := make(map[string]bool)
	for _, secret := range existingSecrets {
		existingKeys[secret.Key] = true
	}

	// Find missing keys
	var missingKeys []string
	for _, key := range expectedKeys {
		if !existingKeys[key] {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) == 0 {
		output.PrintSuccess(fmt.Sprintf("All %d expected %s configured",
			len(expectedKeys),
			output.Plural(len(expectedKeys), "secret", "secrets")))
		return nil
	}

	// Check-only mode: report and exit with error
	if checkOnly {
		output.PrintHeader("Missing Secrets")
		output.PrintNewline()
		for _, key := range missingKeys {
			output.PrintBullet(output.Bold(key))
		}
		output.PrintNewline()
		return fmt.Errorf("%d missing %s; run 'pilot secret configure' to set values",
			len(missingKeys),
			output.Plural(len(missingKeys), "secret", "secrets"))
	}

	// Interactive mode: prompt for each missing secret
	if !h.terminalInput.IsTerminal() {
		return fmt.Errorf("interactive mode requires a terminal; use --check to validate without prompting")
	}

	output.PrintHeader("Configure Missing Secrets")
	output.PrintNewline()

	secrets := existingSecrets
	var added, skipped int
	for _, key := range missingKeys {
		prompt := fmt.Sprintf("  Enter value for %s: ", output.Bold(key))
		value, err := h.terminalInput.ReadPassword(prompt)
		if err != nil {
			return fmt.Errorf("failed to read secret '%s': %w", key, err)
		}

		if value == "" {
			output.PrintStep(fmt.Sprintf("Skipping '%s' (empty value)", key))
			skipped++
			continue
		}

		if conflicting, found := findConflictingSecretKey(secrets, key); found {
			output.PrintStep(fmt.Sprintf("Skipping '%s': conflicts with existing secret '%s' (a secret key cannot have both a direct value and nested keys)", key, conflicting))
			skipped++
			continue
		}

		secrets = append(secrets, &domain.Secret{
			Key:   key,
			Value: value,
		})
		added++
	}

	// Only save if any secrets were added
	if added > 0 {
		err = h.secretsRepository.SaveSecrets(secrets, configContext.Name)
		if err != nil {
			return err
		}
	}

	// Print summary
	output.PrintNewline()
	if added > 0 {
		output.PrintSuccess(fmt.Sprintf("Configured %d %s",
			added, output.Plural(added, "secret", "secrets")))
	}
	if skipped > 0 {
		output.PrintWarning(fmt.Sprintf("Skipped %d %s",
			skipped, output.Plural(skipped, "secret", "secrets")))
	}

	return nil
}

func findConflictingSecretKey(secrets []*domain.Secret, newKey string) (string, bool) {
	for _, secret := range secrets {
		existing := secret.Key
		if existing == newKey {
			continue
		}
		if strings.HasPrefix(newKey, existing+".") || strings.HasPrefix(existing, newKey+".") {
			return existing, true
		}
	}
	return "", false
}
