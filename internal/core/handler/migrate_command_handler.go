package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"pilot/internal/cli/output"
	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

const (
	oldConfigFileName = ".dx-config.yaml"
	newConfigFileName = ".pilot-config.yaml"
	oldDataDirName    = ".dx"
	newDataDirName    = ".pilot"
)

// contextList is a minimal struct for parsing context names from the config file.
type contextList struct {
	Contexts []struct {
		Name string `yaml:"name"`
	} `yaml:"contexts"`
}

type MigrateCommandHandler struct {
	oldKeyring    ports.Keyring
	newKeyring    ports.Keyring
	terminalInput ports.TerminalInput
	fileSystem    ports.FileSystem
}

func NewMigrateCommandHandler(
	oldKeyring ports.Keyring,
	newKeyring ports.Keyring,
	terminalInput ports.TerminalInput,
	fileSystem ports.FileSystem,
) MigrateCommandHandler {
	return MigrateCommandHandler{
		oldKeyring:    oldKeyring,
		newKeyring:    newKeyring,
		terminalInput: terminalInput,
		fileSystem:    fileSystem,
	}
}

// Handle checks for old dx configuration and offers to migrate it.
// Returns true if migration was performed.
func (h *MigrateCommandHandler) Handle() (bool, error) {
	home, err := h.fileSystem.HomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get home directory: %w", err)
	}

	newConfigPath := filepath.Join(home, newConfigFileName)
	oldConfigPath := filepath.Join(home, oldConfigFileName)

	// If new config already exists, no migration needed
	if fileExists(newConfigPath) {
		return false, nil
	}

	// If old config doesn't exist, nothing to migrate
	if !fileExists(oldConfigPath) {
		return false, nil
	}

	// Skip migration prompt in non-interactive contexts (pipes, CI, shell completions)
	if !h.terminalInput.IsTerminal() {
		return false, nil
	}

	// Ask user
	answer, err := h.terminalInput.ReadLine("Found existing dx configuration. Move config, data, and secrets to pilot? This renames ~/.dx-config.yaml and ~/.dx/ in place. [y/N] ")
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return false, nil
	}

	// Parse old config to get context names for keyring migration
	contextNames, err := parseContextNames(oldConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse old config file: %w", err)
	}

	// Check preconditions before mutating
	oldDataDir := filepath.Join(home, oldDataDirName)
	newDataDir := filepath.Join(home, newDataDirName)
	if fileExists(oldDataDir) && fileExists(newDataDir) {
		return false, fmt.Errorf("cannot migrate: ~/%s/ already exists. Remove it manually to retry migration, or delete ~/%s to skip", newDataDirName, oldConfigFileName)
	}

	// Migrate config file
	if err := os.Rename(oldConfigPath, newConfigPath); err != nil {
		return false, fmt.Errorf("failed to migrate config file: %w", err)
	}
	output.PrintSuccess(fmt.Sprintf("Migrated ~/%s to ~/%s", oldConfigFileName, newConfigFileName))

	// Migrate data directory
	if fileExists(oldDataDir) {
		if err := migrateDataDir(oldDataDir, newDataDir); err != nil {
			return false, fmt.Errorf("failed to migrate data directory (config was already moved to ~/%s): %w", newConfigFileName, err)
		}
		output.PrintSuccess(fmt.Sprintf("Migrated ~/%s to ~/%s", oldDataDirName, newDataDirName))
	}

	// Migrate keyring secrets
	migratedKeys, err := h.migrateKeyringSecrets(contextNames)
	if err != nil {
		output.PrintWarning(fmt.Sprintf("failed to migrate some keyring entries: %v", err))
		output.PrintWarningSecondary("if CA keys failed, delete the CA with 'pilot ca delete' so a new one is created on next install")
		output.PrintWarningSecondary("if encryption keys failed, run 'pilot secret configure' to re-enter secrets")
	} else if migratedKeys > 0 {
		output.PrintSuccess(fmt.Sprintf("Migrated %d %s in keyring", migratedKeys, output.Plural(migratedKeys, "secret", "secrets")))
	}

	output.PrintInfo("Migration complete")
	return true, nil
}

func parseContextNames(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath) //nolint:gosec // migration reads user's own config file
	if err != nil {
		return nil, err
	}

	var config contextList
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var names []string
	for _, ctx := range config.Contexts {
		if ctx.Name == "" {
			continue
		}
		if err := domain.ValidateContextName(ctx.Name); err != nil {
			return nil, fmt.Errorf("context name %q: %w", ctx.Name, err)
		}
		names = append(names, ctx.Name)
	}
	return names, nil
}

func migrateDataDir(oldDir, newDir string) error {
	if fileExists(newDir) {
		return fmt.Errorf("target directory ~/%s already exists", newDataDirName)
	}
	return os.Rename(oldDir, newDir)
}

func (h *MigrateCommandHandler) migrateKeyringSecrets(contextNames []string) (int, error) {
	migrated := 0
	var errs []string

	for _, contextName := range contextNames {
		keySuffixes := []string{"-encryption-key", "-ca-key"}
		for _, suffix := range keySuffixes {
			keyName := contextName + suffix
			value, err := h.oldKeyring.GetKey(keyName)
			if err != nil {
				// Key doesn't exist in old keyring, skip
				continue
			}

			if err := h.newKeyring.SetKey(keyName, value); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", keyName, err))
				continue
			}

			if err := h.oldKeyring.DeleteKey(keyName); err != nil {
				// Non-fatal: old key remains but new key was created
				errs = append(errs, fmt.Sprintf("delete old %s: %v", keyName, err))
			}

			migrated++
		}
	}

	if len(errs) > 0 {
		return migrated, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return migrated, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
