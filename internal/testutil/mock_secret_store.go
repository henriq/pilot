package testutil

import (
	"dx/internal/core/domain"
	"dx/internal/ports"

	"github.com/stretchr/testify/mock"
)

var _ ports.SecretStore = (*MockSecretStore)(nil)

type MockSecretStore struct {
	mock.Mock
}

func (m *MockSecretStore) SecretExists(name string) (bool, error) {
	args := m.Called(name)
	return args.Bool(0), args.Error(1)
}

func (m *MockSecretStore) GetSecretData(name string) (map[string][]byte, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (m *MockSecretStore) CreateOrUpdateSecret(name string, secretType domain.K8sSecretType, data map[string][]byte) error {
	args := m.Called(name, secretType, data)
	return args.Error(0)
}

func (m *MockSecretStore) DeleteSecret(name string) error {
	args := m.Called(name)
	return args.Error(0)
}
