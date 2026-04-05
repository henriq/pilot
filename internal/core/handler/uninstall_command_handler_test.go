package handler

import (
	"testing"

	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUninstallCommandHandler_HandleUninstallsAllServices(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"all"},
			},
			{
				Name:         "service-2",
				HelmRepoPath: "any-repo-2",
				HelmBranch:   "any-branch-2",
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("UninstallService", mock.Anything).Return(nil)
	containerOrchestrator.On("HasDeployedServices").Return(false, nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	devProxyManager := core.ProvideDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		configGenerator,
	)
	environmentEnsurer := core.ProvideEnvironmentEnsurer(
		configRepository,
		containerOrchestrator,
	)
	sut := ProvideUninstallCommandHandler(
		configRepository,
		containerOrchestrator,
		environmentEnsurer,
		devProxyManager,
	)

	result := sut.Handle([]string{}, "all")

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
	fileSystem.AssertExpectations(t)
	containerOrchestrator.AssertExpectations(t)
	containerOrchestrator.AssertNumberOfCalls(t, "UninstallService", 3)
}

func TestUninstallCommandHandler_HandleUninstallsOnlySelectedService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"default"},
			},
			{
				Name:         "service-2",
				HelmRepoPath: "any-repo-2",
				HelmBranch:   "any-branch-2",
				Profiles:     []string{"default"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("UninstallService", &configContext.Services[0]).Return(nil)
	containerOrchestrator.On("HasDeployedServices").Return(true, nil)
	fileSystem := new(testutil.MockFileSystem)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	devProxyManager := core.ProvideDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		configGenerator,
	)
	environmentEnsurer := core.ProvideEnvironmentEnsurer(
		configRepository,
		containerOrchestrator,
	)
	sut := ProvideUninstallCommandHandler(
		configRepository,
		containerOrchestrator,
		environmentEnsurer,
		devProxyManager,
	)

	result := sut.Handle([]string{"service-1"}, "default")

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
	containerOrchestrator.AssertExpectations(t)
}
