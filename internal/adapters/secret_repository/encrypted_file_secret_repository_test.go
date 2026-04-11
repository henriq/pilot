package secret_repository

import (
	"errors"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLoadSecrets_Success(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secretsJSON := []byte(`[{"key":"DB_PASSWORD","value":"secret123"}]`)
	encryptedData := []byte("encrypted-data")
	encryptionKey := "test-key"

	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	fileSystem.On("ReadFile", "~/.pilot/test-context/secrets").Return(encryptedData, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(encryptionKey, nil)
	encryptor.On("Decrypt", encryptedData, []byte(encryptionKey)).Return(secretsJSON, nil)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.NoError(t, err)
	assert.Len(t, secrets, 1)
	assert.Equal(t, "DB_PASSWORD", secrets[0].Key)
	assert.Equal(t, "secret123", secrets[0].Value)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestLoadSecrets_FileNotExists(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(false, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.NoError(t, err)
	assert.Empty(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
}

func TestLoadSecrets_KeyNotExists(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(false, nil)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.NoError(t, err)
	assert.Empty(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
}

func TestLoadSecrets_FileExistsError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	expectedErr := errors.New("filesystem error")
	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(false, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, secrets)
	fileSystem.AssertExpectations(t)
}

func TestLoadSecrets_HasKeyError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	expectedErr := errors.New("keyring error")
	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(false, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
}

func TestLoadSecrets_ReadFileError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	expectedErr := errors.New("read file error")
	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	fileSystem.On("ReadFile", "~/.pilot/test-context/secrets").Return([]byte{}, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
}

func TestLoadSecrets_GetKeyError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	expectedErr := errors.New("get key error")
	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	fileSystem.On("ReadFile", "~/.pilot/test-context/secrets").Return([]byte("encrypted"), nil)
	keyring.On("GetKey", "test-context-encryption-key").Return("", expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
}

func TestLoadSecrets_DecryptionError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	expectedErr := errors.New("decryption error")
	encryptedData := []byte("encrypted-data")
	encryptionKey := "test-key"

	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	fileSystem.On("ReadFile", "~/.pilot/test-context/secrets").Return(encryptedData, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(encryptionKey, nil)
	encryptor.On("Decrypt", encryptedData, []byte(encryptionKey)).Return([]byte{}, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestLoadSecrets_InvalidJSON(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	invalidJSON := []byte("not valid json")
	encryptedData := []byte("encrypted-data")
	encryptionKey := "test-key"

	fileSystem.On("FileExists", "~/.pilot/test-context/secrets").Return(true, nil)
	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	fileSystem.On("ReadFile", "~/.pilot/test-context/secrets").Return(encryptedData, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(encryptionKey, nil)
	encryptor.On("Decrypt", encryptedData, []byte(encryptionKey)).Return(invalidJSON, nil)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	secrets, err := sut.LoadSecrets("test-context")

	assert.Error(t, err)
	assert.Nil(t, secrets)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestSaveSecrets_SuccessWithExistingKey(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	encryptionKey := "test-key"
	encryptedData := []byte("encrypted-data")

	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(encryptionKey, nil)
	encryptor.On("Encrypt", mock.Anything, []byte(encryptionKey)).Return(encryptedData, nil)
	fileSystem.On("WriteFile", "~/.pilot/test-context/secrets", encryptedData, mock.Anything).Return(nil)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.NoError(t, err)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestSaveSecrets_SuccessWithNewKey(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	newKey := []byte("new-encryption-key")
	encryptedData := []byte("encrypted-data")

	keyring.On("HasKey", "test-context-encryption-key").Return(false, nil)
	encryptor.On("CreateKey").Return(newKey, nil)
	keyring.On("SetKey", "test-context-encryption-key", string(newKey)).Return(nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(string(newKey), nil)
	encryptor.On("Encrypt", mock.Anything, newKey).Return(encryptedData, nil)
	fileSystem.On("WriteFile", "~/.pilot/test-context/secrets", encryptedData, mock.Anything).Return(nil)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.NoError(t, err)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestSaveSecrets_HasKeyError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	expectedErr := errors.New("has key error")

	keyring.On("HasKey", "test-context-encryption-key").Return(false, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	keyring.AssertExpectations(t)
}

func TestSaveSecrets_CreateKeyError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	expectedErr := errors.New("create key error")

	keyring.On("HasKey", "test-context-encryption-key").Return(false, nil)
	encryptor.On("CreateKey").Return([]byte{}, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestSaveSecrets_SetKeyError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	newKey := []byte("new-encryption-key")
	expectedErr := errors.New("set key error")

	keyring.On("HasKey", "test-context-encryption-key").Return(false, nil)
	encryptor.On("CreateKey").Return(newKey, nil)
	keyring.On("SetKey", "test-context-encryption-key", string(newKey)).Return(expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestSaveSecrets_GetKeyError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	expectedErr := errors.New("get key error")

	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return("", expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	keyring.AssertExpectations(t)
}

func TestSaveSecrets_EncryptionError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	encryptionKey := "test-key"
	expectedErr := errors.New("encryption error")

	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(encryptionKey, nil)
	encryptor.On("Encrypt", mock.Anything, []byte(encryptionKey)).Return([]byte{}, expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestSaveSecrets_WriteError(t *testing.T) {
	fileSystem := new(testutil.MockFileSystem)
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}
	encryptionKey := "test-key"
	encryptedData := []byte("encrypted-data")
	expectedErr := errors.New("write error")

	keyring.On("HasKey", "test-context-encryption-key").Return(true, nil)
	keyring.On("GetKey", "test-context-encryption-key").Return(encryptionKey, nil)
	encryptor.On("Encrypt", mock.Anything, []byte(encryptionKey)).Return(encryptedData, nil)
	fileSystem.On("WriteFile", "~/.pilot/test-context/secrets", encryptedData, mock.Anything).Return(expectedErr)

	sut := NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)

	err := sut.SaveSecrets(secrets, "test-context")

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	fileSystem.AssertExpectations(t)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}
