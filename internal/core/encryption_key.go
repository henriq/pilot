package core

import (
	"fmt"

	"pilot/internal/ports"
)

// GetOrCreateEncryptionKey retrieves an encryption key from the keyring,
// creating one if it doesn't exist.
func GetOrCreateEncryptionKey(
	keyring ports.Keyring,
	encryptor ports.SymmetricEncryptor,
	keyName string,
) (string, error) {
	hasKey, err := keyring.HasKey(keyName)
	if err != nil {
		return "", fmt.Errorf("failed to check keyring for key '%s': %w", keyName, err)
	}

	if !hasKey {
		key, err := encryptor.CreateKey()
		if err != nil {
			return "", fmt.Errorf("failed to create encryption key '%s': %w", keyName, err)
		}
		if err := keyring.SetKey(keyName, string(key)); err != nil {
			return "", fmt.Errorf("failed to store encryption key '%s': %w", keyName, err)
		}
	}

	passphrase, err := keyring.GetKey(keyName)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve encryption key '%s': %w", keyName, err)
	}
	return passphrase, nil
}
