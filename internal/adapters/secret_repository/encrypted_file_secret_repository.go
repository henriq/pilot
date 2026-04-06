package secret_repository

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/ports"
)

var _ ports.SecretsRepository = (*EncryptedFileSecretRepository)(nil)

type EncryptedFileSecretRepository struct {
	fileSystem ports.FileSystem
	keyring    ports.Keyring
	encryptor  ports.SymmetricEncryptor
}

func ProvideEncryptedFileSecretRepository(
	fileSystem ports.FileSystem,
	keyring ports.Keyring,
	encryptor ports.SymmetricEncryptor,
) *EncryptedFileSecretRepository {
	return &EncryptedFileSecretRepository{
		fileSystem: fileSystem,
		keyring:    keyring,
		encryptor:  encryptor,
	}
}

func (e *EncryptedFileSecretRepository) LoadSecrets(configContextName string) ([]*domain.Secret, error) {
	secretsFilePath := filepath.Join("~", ".dx", configContextName, "secrets")
	secretFileExists, err := e.fileSystem.FileExists(secretsFilePath)
	if err != nil {
		return nil, err
	}
	keyExists, err := e.keyring.HasKey(fmt.Sprintf("%s-encryption-key", configContextName))
	if err != nil {
		return nil, err
	}
	if !secretFileExists || !keyExists {
		return []*domain.Secret{}, nil
	}

	encryptedSecrets, err := e.fileSystem.ReadFile(secretsFilePath)
	if err != nil {
		return nil, err
	}

	key, err := e.keyring.GetKey(fmt.Sprintf("%s-encryption-key", configContextName))
	if err != nil {
		return nil, err
	}

	decryptedSecrets, err := e.encryptor.Decrypt(encryptedSecrets, []byte(key))

	if err != nil {
		return nil, err
	}

	var secrets []*domain.Secret
	err = json.Unmarshal(decryptedSecrets, &secrets)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}

func (e *EncryptedFileSecretRepository) SaveSecrets(
	secrets []*domain.Secret,
	configContextName string,
) error {
	secretsFilePath := filepath.Join("~", ".dx", configContextName, "secrets")
	key, err := core.GetOrCreateEncryptionKey(e.keyring, e.encryptor, fmt.Sprintf("%s-encryption-key", configContextName))
	if err != nil {
		return err
	}

	secretBytes, err := json.Marshal(secrets)
	if err != nil {
		return err
	}

	encryptedSecrets, err := e.encryptor.Encrypt(secretBytes, []byte(key))
	if err != nil {
		return err
	}

	err = e.fileSystem.WriteFile(secretsFilePath, encryptedSecrets, ports.ReadWrite)
	if err != nil {
		return err
	}

	return nil
}
