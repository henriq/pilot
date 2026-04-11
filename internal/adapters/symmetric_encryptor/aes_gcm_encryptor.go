package symmetric_encryptor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"pilot/internal/ports"
)

var _ ports.SymmetricEncryptor = (*AesGcmEncryptor)(nil)

type AesGcmEncryptor struct{}

func NewAesGcmEncryptor() *AesGcmEncryptor {
	return &AesGcmEncryptor{}
}

func (a AesGcmEncryptor) Encrypt(plaintext []byte, encodedKey []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(string(encodedKey))
	if err != nil {
		return nil, err
	}
	// Create a new cipher block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create a new GCM cipher mode using the block
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a nonce (number used once) for this encryption
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the plaintext and append the nonce to the ciphertext
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	return []byte(base64.StdEncoding.EncodeToString(ciphertext)), nil
}

func (a AesGcmEncryptor) Decrypt(encodedCipherText []byte, encodedKey []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(string(encodedKey))
	if err != nil {
		return nil, err
	}
	cipherText, err := base64.StdEncoding.DecodeString(string(encodedCipherText))
	if err != nil {
		return nil, err
	}

	// Create a new cipher block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create a new GCM cipher mode using the block
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Get the nonce size
	nonceSize := aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract the nonce from the ciphertext
	nonce, ciphertext := cipherText[:nonceSize], cipherText[nonceSize:]

	// Decrypt the ciphertext
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (a AesGcmEncryptor) CreateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return []byte(base64.StdEncoding.EncodeToString(key)), nil
}
