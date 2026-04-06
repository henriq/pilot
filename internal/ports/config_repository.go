package ports

import "dx/internal/core/domain"

type ConfigRepository interface {
	LoadConfig() (*domain.Config, error)
	SaveConfig(*domain.Config) error
	ConfigExists() (bool, error)
	LoadCurrentConfigurationContext() (*domain.ConfigurationContext, error)
	LoadCurrentContextName() (string, error)
	SaveCurrentContextName(string) error
	LoadEnvKey(contextName string) (string, error)
}
