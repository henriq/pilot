package testutil

import (
	"pilot/internal/core/domain"

	"github.com/stretchr/testify/mock"
)

type MockConfigRepository struct {
	mock.Mock
}

func (m *MockConfigRepository) LoadCurrentConfigurationContext() (*domain.ConfigurationContext, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ConfigurationContext), args.Error(1)
}

func (m *MockConfigRepository) LoadCurrentContextName() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigRepository) SaveCurrentContextName(contextName string) error {
	args := m.Called(contextName)
	return args.Error(0)
}

func (m *MockConfigRepository) LoadEnvKey(contextName string) (string, error) {
	args := m.Called(contextName)
	return args.String(0), args.Error(1)
}

func (m *MockConfigRepository) LoadConfig() (*domain.Config, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Config), args.Error(1)
}

func (m *MockConfigRepository) SaveConfig(config *domain.Config) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockConfigRepository) ConfigExists() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}
