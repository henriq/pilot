package core

import (
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestEnvironmentEnsurer_EnsureExpectedClusterIsSelectedReturnsTrueWhenEnvKeysMatch(t *testing.T) {
	expectedEnvKey := string(uuid.NewUUID())
	configContext := domain.ConfigurationContext{
		Name: string(uuid.NewUUID()),
	}
	containerOrchestrator := testutil.MockContainerOrchestrator{}
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return(expectedEnvKey, nil)
	configRepository := testutil.MockConfigRepository{}
	configRepository.On("LoadEnvKey", mock.Anything).Return(expectedEnvKey, nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(&configContext, nil)
	sut := NewEnvironmentEnsurer(
		&configRepository,
		&containerOrchestrator,
	)

	result := sut.EnsureExpectedClusterIsSelected()

	assert.Nil(t, result)

}

func TestEnvironmentEnsurer_EnsureExpectedClusterIsSelectedReturnsFalseWhenEnvKeysDontMatch(t *testing.T) {
	expectedEnvKey := string(uuid.NewUUID())
	configContext := domain.ConfigurationContext{
		Name: string(uuid.NewUUID()),
	}
	containerOrchestrator := testutil.MockContainerOrchestrator{}
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return(expectedEnvKey, nil)
	configRepository := testutil.MockConfigRepository{}
	configRepository.On("LoadEnvKey", mock.Anything).Return(string(uuid.NewUUID()), nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(&configContext, nil)
	sut := NewEnvironmentEnsurer(
		&configRepository,
		&containerOrchestrator,
	)

	result := sut.EnsureExpectedClusterIsSelected()

	assert.NotNil(t, result)
}
