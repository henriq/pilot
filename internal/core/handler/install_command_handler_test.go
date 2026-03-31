package handler

import (
	"testing"

	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInstallCommandHandler_HandleInstallsAllServices(t *testing.T) {
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
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.InterceptHttp == false
	})).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		configContext.Services[0].HelmPath,
	).Return(nil)
	scm.On(
		"Download",
		configContext.Services[1].HelmRepoPath,
		configContext.Services[1].HelmBranch,
		configContext.Services[1].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil) // No existing deployment, will trigger rebuild
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
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
	)

	result := sut.Handle([]string{}, "all", false)

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 1) // Only HAProxy (no mitmproxy without --intercept-http)
	fileSystem.AssertExpectations(t)
	containerOrchestrator.AssertExpectations(t)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 2)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 1)
	scm.AssertNumberOfCalls(t, "Download", 2)
	scm.AssertExpectations(t)
}

func TestInstallCommandHandler_HandleInstallsOnlySelectedService(t *testing.T) {
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
	containerOrchestrator.On("InstallService", mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil) // No existing deployment, will trigger rebuild
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
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
	)

	result := sut.Handle([]string{"service-1"}, "all", false)

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 1) // Only HAProxy
	fileSystem.AssertExpectations(t)
	containerOrchestrator.AssertExpectations(t)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 1)
	scm.AssertNumberOfCalls(t, "Download", 1)
	scm.AssertExpectations(t)
}

func TestInstallCommandHandler_HandleWithInterceptHttp(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.InterceptHttp == true
	})).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil) // No existing deployment, will trigger rebuild
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
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
	)

	result := sut.Handle([]string{}, "all", true)

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 2) // HAProxy + mitmproxy when --intercept-http
	fileSystem.AssertExpectations(t)
	containerOrchestrator.AssertExpectations(t)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 1)
	scm.AssertNumberOfCalls(t, "Download", 1)
	scm.AssertExpectations(t)
}

func TestInstallCommandHandler_HandleSkipsDevProxyWhenChecksumUnchanged(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		LocalServices: []domain.LocalService{
			{
				Name:            "test-service",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
			},
		},
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"default"},
			},
		},
	}
	// Calculate the expected checksum for the LocalServices (without interception)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	expectedChecksum := configGenerator.GenerateChecksum(configContext, false)

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything).Return(nil)
	// Return matching checksum - dev-proxy should be skipped
	containerOrchestrator.On("GetDevProxyChecksum").Return(expectedChecksum, nil)
	fileSystem := new(testutil.MockFileSystem)
	// FileSystem WriteFile should NOT be called since dev-proxy is skipped
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	// BuildImage should NOT be called since dev-proxy is skipped

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
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
	)

	result := sut.Handle([]string{}, "default", false)

	assert.Nil(t, result)
	// Verify BuildImage was NOT called since dev-proxy checksum matched
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 0)
	// Verify InstallDevProxy was NOT called
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 0)
	// Verify WriteFile was NOT called for dev-proxy config
	fileSystem.AssertNumberOfCalls(t, "WriteFile", 0)
	// Verify user service was still installed
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	scm.AssertNumberOfCalls(t, "Download", 1)
}

func TestInstallCommandHandler_HandleSkipsDevProxyWhenChecksumUnchanged_WithInterceptHttp(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		LocalServices: []domain.LocalService{
			{
				Name:            "test-service",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
			},
		},
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"default"},
			},
		},
	}
	// Calculate the expected checksum with interception enabled
	configGenerator := core.ProvideDevProxyConfigGenerator()
	expectedChecksum := configGenerator.GenerateChecksum(configContext, true)

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything).Return(nil)
	// Return matching checksum for interceptHttp=true — dev-proxy should be skipped
	containerOrchestrator.On("GetDevProxyChecksum").Return(expectedChecksum, nil)
	fileSystem := new(testutil.MockFileSystem)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)

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
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
	)

	result := sut.Handle([]string{}, "default", true)

	assert.Nil(t, result)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 0)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 0)
	fileSystem.AssertNumberOfCalls(t, "WriteFile", 0)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	scm.AssertNumberOfCalls(t, "Download", 1)
}

func TestInstallCommandHandler_HandleReturnsErrorFromShouldRebuildDevProxy(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"default"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", assert.AnError)
	fileSystem := new(testutil.MockFileSystem)
	scm := new(testutil.MockScm)
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
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
	)

	result := sut.Handle([]string{}, "default", false)

	assert.Error(t, result)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 0)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 0)
}
