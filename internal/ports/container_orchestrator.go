package ports

import (
	"dx/internal/core/domain"
)

type ContainerOrchestrator interface {
	CreateClusterEnvironmentKey() (string, error)
	InstallService(service *domain.Service, certificateSecrets []byte) error
	InstallDevProxy(service *domain.Service, certificateSecrets []byte) error
	UninstallService(service *domain.Service) error
	HasDeployedServices() (bool, error)
	// GetDevProxyChecksum returns the checksum annotation from the existing dev-proxy deployment.
	// Returns an empty string if the deployment doesn't exist.
	GetDevProxyChecksum() (string, error)
}
