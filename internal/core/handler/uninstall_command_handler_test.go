package handler

import (
	"testing"

	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/testutil"

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
	containerOrchestrator.On("UninstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.Name == "service-1"
	})).Return(nil).Once()
	containerOrchestrator.On("UninstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.Name == "service-2"
	})).Return(nil).Once()
	containerOrchestrator.On("UninstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.Name == "dev-proxy"
	})).Return(nil).Once()
	containerOrchestrator.On("HasDeployedServices").Return(false, nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.NewDevProxyConfigGenerator()
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		configGenerator,
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(
		configRepository,
		containerOrchestrator,
	)
	sut := NewUninstallCommandHandler(
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
	configGenerator := core.NewDevProxyConfigGenerator()
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		configGenerator,
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(
		configRepository,
		containerOrchestrator,
	)
	sut := NewUninstallCommandHandler(
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

func TestUninstallCommandHandler_Handle_EnsureExpectedClusterError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("", assert.AnError)
	fileSystem := new(testutil.MockFileSystem)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.NewDevProxyConfigGenerator()
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		configGenerator,
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(
		configRepository,
		containerOrchestrator,
	)
	sut := NewUninstallCommandHandler(
		configRepository,
		containerOrchestrator,
		environmentEnsurer,
		devProxyManager,
	)

	result := sut.Handle([]string{}, "all")

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "failed to generate current environment key")
	containerOrchestrator.AssertNotCalled(t, "UninstallService", mock.Anything)
}

func TestUninstallCommandHandler_Handle_LoadConfigError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	// First call succeeds (inside EnsureExpectedClusterIsSelected), second call fails (in Handle)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil).Once()
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, assert.AnError).Once()
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	fileSystem := new(testutil.MockFileSystem)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.NewDevProxyConfigGenerator()
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		configGenerator,
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(
		configRepository,
		containerOrchestrator,
	)
	sut := NewUninstallCommandHandler(
		configRepository,
		containerOrchestrator,
		environmentEnsurer,
		devProxyManager,
	)

	result := sut.Handle([]string{}, "all")

	assert.ErrorIs(t, result, assert.AnError)
	containerOrchestrator.AssertNotCalled(t, "UninstallService", mock.Anything)
}
