package ports

import (
	"pilot/internal/core/domain"
)

type ContainerImageRepository interface {
	BuildImage(image domain.DockerImage, configContext *domain.ConfigurationContext) error
	PullImage(image string) error
}
