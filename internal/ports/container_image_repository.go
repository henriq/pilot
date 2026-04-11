package ports

import (
	"pilot/internal/core/domain"
)

type ContainerImageRepository interface {
	BuildImage(image domain.DockerImage) error
	PullImage(image string) error
}
