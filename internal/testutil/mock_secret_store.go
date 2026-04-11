package testutil

import (
	"pilot/internal/ports"

	"github.com/stretchr/testify/mock"
)

var _ ports.SecretStore = (*MockSecretStore)(nil)

type MockSecretStore struct {
	mock.Mock
}

func (m *MockSecretStore) GetSecretData(name string) (map[string][]byte, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]byte), args.Error(1)
}
