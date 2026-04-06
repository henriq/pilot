package keyring

import (
	"dx/internal/ports"
	"errors"

	"github.com/zalando/go-keyring"
)

const keyringService = "se.henriq.dx"

var _ ports.Keyring = (*ZalandoKeyring)(nil)

type ZalandoKeyring struct{}

func ProvideZalandoKeyring() *ZalandoKeyring {
	return &ZalandoKeyring{}
}

func (z ZalandoKeyring) GetKey(keyName string) (string, error) {
	return keyring.Get(keyringService, keyName)
}

func (z ZalandoKeyring) SetKey(keyName string, keyValue string) error {
	return keyring.Set(keyringService, keyName, keyValue)
}

func (z ZalandoKeyring) HasKey(keyName string) (bool, error) {
	_, err := keyring.Get(keyringService, keyName)
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (z ZalandoKeyring) DeleteKey(keyName string) error {
	err := keyring.Delete(keyringService, keyName)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
