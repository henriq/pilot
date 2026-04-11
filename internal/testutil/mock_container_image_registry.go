package testutil

import (
	"pilot/internal/core/domain"
	"pilot/internal/ports"

	"github.com/stretchr/testify/mock"
)

// Compile-time interface compliance check
var _ ports.ContainerImageRepository = (*MockContainerImageRepository)(nil)

type MockContainerImageRepository struct {
	mock.Mock
}

func (m *MockContainerImageRepository) BuildImage(image domain.DockerImage) error {
	args := m.Called(image)
	return args.Error(0)
}

func (m *MockContainerImageRepository) PullImage(image string) error {
	args := m.Called(image)
	return args.Error(0)
}
