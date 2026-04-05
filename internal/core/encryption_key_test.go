package core

import (
	"testing"

	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrCreateEncryptionKey_ReturnsExistingKey(t *testing.T) {
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	keyring.On("HasKey", "test-key").Return(true, nil)
	keyring.On("GetKey", "test-key").Return("existing-passphrase", nil)

	result, err := GetOrCreateEncryptionKey(keyring, encryptor, "test-key")

	require.NoError(t, err)
	assert.Equal(t, "existing-passphrase", result)
	encryptor.AssertNotCalled(t, "CreateKey")
	keyring.AssertNotCalled(t, "SetKey")
}

func TestGetOrCreateEncryptionKey_CreatesNewKey(t *testing.T) {
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	keyring.On("HasKey", "test-key").Return(false, nil)
	encryptor.On("CreateKey").Return([]byte("new-key"), nil)
	keyring.On("SetKey", "test-key", "new-key").Return(nil)
	keyring.On("GetKey", "test-key").Return("new-key", nil)

	result, err := GetOrCreateEncryptionKey(keyring, encryptor, "test-key")

	require.NoError(t, err)
	assert.Equal(t, "new-key", result)
	keyring.AssertExpectations(t)
	encryptor.AssertExpectations(t)
}

func TestGetOrCreateEncryptionKey_HasKeyError(t *testing.T) {
	keyring := new(testutil.MockKeyring)

	keyring.On("HasKey", "test-key").Return(false, assert.AnError)

	_, err := GetOrCreateEncryptionKey(keyring, nil, "test-key")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to check keyring for key")
}

func TestGetOrCreateEncryptionKey_CreateKeyError(t *testing.T) {
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	keyring.On("HasKey", "test-key").Return(false, nil)
	encryptor.On("CreateKey").Return(nil, assert.AnError)

	_, err := GetOrCreateEncryptionKey(keyring, encryptor, "test-key")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to create encryption key")
}

func TestGetOrCreateEncryptionKey_SetKeyError(t *testing.T) {
	keyring := new(testutil.MockKeyring)
	encryptor := new(testutil.MockSymmetricEncryptor)

	keyring.On("HasKey", "test-key").Return(false, nil)
	encryptor.On("CreateKey").Return([]byte("new-key"), nil)
	keyring.On("SetKey", "test-key", "new-key").Return(assert.AnError)

	_, err := GetOrCreateEncryptionKey(keyring, encryptor, "test-key")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to store encryption key")
}

func TestGetOrCreateEncryptionKey_GetKeyError(t *testing.T) {
	keyring := new(testutil.MockKeyring)

	keyring.On("HasKey", "test-key").Return(true, nil)
	keyring.On("GetKey", "test-key").Return("", assert.AnError)

	_, err := GetOrCreateEncryptionKey(keyring, nil, "test-key")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to retrieve encryption key")
}
