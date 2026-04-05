package ports

import (
	"dx/internal/core/domain"
)

// SecretStore abstracts Kubernetes secret operations for certificate provisioning.
type SecretStore interface {
	// SecretExists checks whether a Kubernetes secret exists by name in the current namespace.
	SecretExists(name string) (bool, error)
	// GetSecretData returns the data from a Kubernetes secret by name.
	GetSecretData(name string) (map[string][]byte, error)
	// CreateOrUpdateSecret creates or updates a Kubernetes secret.
	// Only overwrites secrets with the managed-by=dx label.
	CreateOrUpdateSecret(name string, secretType domain.K8sSecretType, data map[string][]byte) error
	// DeleteSecret deletes a Kubernetes secret by name.
	// Only deletes secrets with the managed-by=dx label. Returns nil if the secret does not exist.
	DeleteSecret(name string) error
}
