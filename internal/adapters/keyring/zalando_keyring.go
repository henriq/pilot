package keyring

import (
	"errors"
	"pilot/internal/ports"

	"github.com/zalando/go-keyring"
)

var _ ports.Keyring = (*ZalandoKeyring)(nil)

type ZalandoKeyring struct {
	serviceName string
}

func NewZalandoKeyring(serviceName string) *ZalandoKeyring {
	return &ZalandoKeyring{serviceName: serviceName}
}

func (z ZalandoKeyring) GetKey(keyName string) (string, error) {
	return keyring.Get(z.serviceName, keyName)
}

func (z ZalandoKeyring) SetKey(keyName string, keyValue string) error {
	return keyring.Set(z.serviceName, keyName, keyValue)
}

func (z ZalandoKeyring) HasKey(keyName string) (bool, error) {
	_, err := keyring.Get(z.serviceName, keyName)
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (z ZalandoKeyring) DeleteKey(keyName string) error {
	err := keyring.Delete(z.serviceName, keyName)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
