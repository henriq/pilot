package testutil

import (
	"pilot/internal/core/domain"

	"github.com/stretchr/testify/mock"
)

type MockSecretsRepository struct {
	mock.Mock
}

func (m *MockSecretsRepository) LoadSecrets(configContextName string) ([]*domain.Secret, error) {
	args := m.Called(configContextName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Secret), args.Error(1)
}

func (m *MockSecretsRepository) SaveSecrets(secrets []*domain.Secret, configContextName string) error {
	args := m.Called(secrets, configContextName)
	return args.Error(0)
}
