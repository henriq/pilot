package ports

import "pilot/internal/core/domain"

type SecretsRepository interface {
	LoadSecrets(configContextName string) ([]*domain.Secret, error)
	SaveSecrets(secrets []*domain.Secret, configContextName string) error
}
