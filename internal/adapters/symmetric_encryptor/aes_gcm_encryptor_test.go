package symmetric_encryptor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestAesGcmEncryptor_EncryptReturnsCipherText(t *testing.T) {
	sut := ProvideAesGcmEncryptor()
	plainText := []byte(uuid.NewUUID())
	key, _ := sut.CreateKey()

	result, err := sut.Encrypt(plainText, key)
	assert.Nil(t, err, err)
	assert.NotEqual(t, plainText, result)
}

func TestAesGcmEncryptor_EncryptReturnsDifferentCipherTextsEachTime(t *testing.T) {
	sut := ProvideAesGcmEncryptor()
	plainText := []byte(uuid.NewUUID())
	key, _ := sut.CreateKey()

	cipherText1, _ := sut.Encrypt(plainText, key)
	cipherText2, _ := sut.Encrypt(plainText, key)

	assert.NotEqual(t, cipherText1, cipherText2)
}

func TestAesGcmEncryptor_DecryptReturnsPlainText(t *testing.T) {
	sut := ProvideAesGcmEncryptor()
	plainText := []byte(uuid.NewUUID())
	key, _ := sut.CreateKey()
	cipherText, _ := sut.Encrypt(plainText, key)

	result, err := sut.Decrypt(cipherText, key)

	assert.Nil(t, err, err)
	assert.Equal(t, plainText, result)
}

func TestAesGcmEncryptor_DecryptWithWrongKeyReturnsError(t *testing.T) {
	sut := ProvideAesGcmEncryptor()
	plainText := []byte(uuid.NewUUID())
	key1, _ := sut.CreateKey()
	key2, _ := sut.CreateKey()
	cipherText, _ := sut.Encrypt(plainText, key1)
	result, err := sut.Decrypt(cipherText, key2)

	assert.NotNil(t, err)
	assert.Nil(t, result)
}
