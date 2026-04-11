package handler

import (
	"fmt"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestProvideGenEnvKeyCommandHandler_WritesEnvKey(t *testing.T) {
	expectedEnvKey := string(uuid.NewUUID())
	configContext := &domain.ConfigurationContext{Name: string(uuid.NewUUID())}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	mockFileSystem := new(testutil.MockFileSystem)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return(expectedEnvKey, nil)
	sut := GenEnvKeyCommandHandler{
		configRepository:      configRepository,
		fileSystem:            mockFileSystem,
		containerOrchestrator: containerOrchestrator,
	}
	mockFileSystem.On("WriteFile", mock.Anything, []byte(expectedEnvKey), mock.Anything).Return(nil)

	result := sut.Handle()

	assert.Nil(t, result)
	mockFileSystem.AssertExpectations(t)
}

func TestProvideGenEnvKeyCommandHandler_WritesNoEnvKeyIfKeyGenerationFailed(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: string(uuid.NewUUID())}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	mockFileSystem := new(testutil.MockFileSystem)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("", fmt.Errorf("error"))
	sut := NewGenEnvKeyCommandHandler(
		configRepository,
		mockFileSystem,
		containerOrchestrator,
	)

	result := sut.Handle()

	assert.NotNil(t, result)
	mockFileSystem.AssertNotCalled(t, "WriteFile")
}
