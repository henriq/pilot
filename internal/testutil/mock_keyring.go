package testutil

import (
	"pilot/internal/ports"

	"github.com/stretchr/testify/mock"
)

var _ ports.Keyring = (*MockKeyring)(nil)

// MockKeyring provides a testify mock for ports.Keyring
type MockKeyring struct {
	mock.Mock
}

func (m *MockKeyring) GetKey(keyName string) (string, error) {
	args := m.Called(keyName)
	return args.String(0), args.Error(1)
}

func (m *MockKeyring) SetKey(keyName string, keyValue string) error {
	args := m.Called(keyName, keyValue)
	return args.Error(0)
}

func (m *MockKeyring) HasKey(keyName string) (bool, error) {
	args := m.Called(keyName)
	return args.Bool(0), args.Error(1)
}

func (m *MockKeyring) DeleteKey(keyName string) error {
	args := m.Called(keyName)
	return args.Error(0)
}
