package handler

import (
	"bytes"
	"io"
	"os"
	"testing"

	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// noCertProvisioner creates a CertificateProvisioner with unconfigured mocks.
// Only safe for test paths that never invoke provisioning (e.g., error paths
// that return before reaching certificate provisioning).
func noCertProvisioner() *core.CertificateProvisioner {
	return core.NewCertificateProvisioner(
		new(testutil.MockCertificateAuthority),
		new(testutil.MockSecretStore),
		new(testutil.MockKeyring),
		new(testutil.MockSymmetricEncryptor),
	)
}

// internalTLSProvisioner creates a CertificateProvisioner with mocks set up
// to handle the internal TLS certificate provisioning that always runs during install.
func internalTLSProvisioner(contextName string) *core.CertificateProvisioner {
	mockCA := new(testutil.MockCertificateAuthority)
	mockSecretStore := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	keyName := contextName + "-ca-key"
	mockKeyring.On("HasKey", keyName).Return(true, nil)
	mockKeyring.On("GetKey", keyName).Return("test-pass", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	return core.NewCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)
}

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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.InterceptHttp == false
	}), mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(nil)
	scm.On(
		"Download",
		configContext.Services[1].HelmRepoPath,
		configContext.Services[1].HelmBranch,
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	configGenerator := core.NewDevProxyConfigGenerator()
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil) // No existing deployment, will trigger rebuild
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", false, domain.HelmChartOverrides{})

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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	configGenerator := core.NewDevProxyConfigGenerator()
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil) // No existing deployment, will trigger rebuild
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{"service-1"}, "all", false, domain.HelmChartOverrides{})

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

func TestInstallCommandHandler_HandleSkipsNamedNonDeployableService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "deployable",
				HelmRepoPath: "any-repo",
				HelmBranch:   "any-branch",
				Profiles:     []string{"all"},
			},
			{
				Name:     "automation",
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		core.NewDevProxyConfigGenerator(),
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		new(testutil.MockScm),
		internalTLSProvisioner("Test"),
	)

	err := sut.Handle([]string{"automation"}, "all", false, domain.HelmChartOverrides{})

	assert.NoError(t, err)
	containerOrchestrator.AssertNotCalled(t, "InstallService", mock.Anything, mock.Anything)
}

func TestInstallCommandHandler_TrackerExcludesNonDeployableService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "deployable",
				HelmRepoPath: "any-repo",
				HelmBranch:   "any-branch",
				Profiles:     []string{"all"},
			},
			{
				Name:     "automation",
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.Name == "deployable"
	}), mock.Anything).Return(nil).Once()
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		core.NewDevProxyConfigGenerator(),
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	oldStdout := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer
	err := sut.Handle([]string{"deployable", "automation"}, "all", false, domain.HelmChartOverrides{})
	writer.Close() //nolint:errcheck,gosec // test pipe close
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, reader)
	require.NoError(t, copyErr)
	captured := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, captured, "Installing deployable")
	assert.NotContains(t, captured, "Installing automation")
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	containerOrchestrator.AssertNotCalled(t, "InstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.Name == "automation"
	}), mock.Anything)
}

func TestInstallCommandHandler_HandleSkipsNonDeployableServicesByProfile(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "deployable",
				HelmRepoPath: "any-repo",
				HelmBranch:   "any-branch",
				Profiles:     []string{"all"},
			},
			{
				Name:     "automation",
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	devProxyManager := core.NewDevProxyManager(
		configRepository,
		fileSystem,
		containerImageRepository,
		containerOrchestrator,
		core.NewDevProxyConfigGenerator(),
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	err := sut.Handle(nil, "all", false, domain.HelmChartOverrides{})

	assert.NoError(t, err)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(s *domain.Service) bool {
		return s.InterceptHttp == true
	}), mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	configGenerator := core.NewDevProxyConfigGenerator()
	// No GetDevProxyChecksum mock needed — interceptHttp always triggers rebuild
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", true, domain.HelmChartOverrides{})

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 2) // HAProxy + mitmproxy when --intercept-http
	fileSystem.AssertExpectations(t)
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
	configGenerator := core.NewDevProxyConfigGenerator()
	expectedChecksum := configGenerator.GenerateChecksum(configContext, false)

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	// Return matching checksum - dev-proxy should not be rebuilt
	containerOrchestrator.On("GetDevProxyChecksum").Return(expectedChecksum, nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("FileExists", "~/.pilot/Test/dev-proxy/helm").Return(true, nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)

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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "default", false, domain.HelmChartOverrides{})

	assert.Nil(t, result)
	// Verify BuildImage was NOT called since dev-proxy checksum matched
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 0)
	// Verify WriteFile was NOT called for dev-proxy config
	fileSystem.AssertNumberOfCalls(t, "WriteFile", 0)
	// Verify dev-proxy was still installed (Helm) to apply cert secrets
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 1)
	// Verify user service was still installed
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	scm.AssertNumberOfCalls(t, "Download", 1)
}

func TestInstallCommandHandler_HandleAlwaysRebuildsDevProxyWithInterceptHttp(t *testing.T) {
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
	configGenerator := core.NewDevProxyConfigGenerator()

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	// Even though checksum would match, dev-proxy should always rebuild with interceptHttp
	result := sut.Handle([]string{}, "default", true, domain.HelmChartOverrides{})

	assert.Nil(t, result)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 2) // HAProxy + mitmproxy
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 1)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
	containerOrchestrator.AssertNotCalled(t, "GetDevProxyChecksum")
}

func TestInstallCommandHandler_HandleProvisionsCertificatesDuringInstall(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"all"},
				Certificates: []domain.CertificateRequest{
					{
						Type:     domain.CertificateTypeServer,
						DNSNames: []string{"foo.localhost"},
						K8sSecret: domain.K8sSecretConfig{
							Name: "foo-tls",
							Type: domain.K8sSecretTypeTLS,
						},
					},
				},
			},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)

	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)

	mockSecretStore := new(testutil.MockSecretStore)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	mockKeyring.On("HasKey", "Test-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "Test-ca-key").Return("test-pass", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	provisioner := core.NewCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)

	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	configGenerator := core.NewDevProxyConfigGenerator()
	devProxyManager := core.NewDevProxyManager(
		configRepository, fileSystem, containerImageRepository, containerOrchestrator, configGenerator,
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewInstallCommandHandler(
		configRepository, containerImageRepository, containerOrchestrator,
		devProxyManager, environmentEnsurer, scm, provisioner,
	)

	result := sut.Handle([]string{}, "all", false, domain.HelmChartOverrides{})

	assert.Nil(t, result)
	mockCA.AssertExpectations(t)
}

func TestInstallCommandHandler_HandleReturnsErrorWhenCertificateProvisioningFails(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				Profiles:     []string{"all"},
				Certificates: []domain.CertificateRequest{
					{
						Type:     domain.CertificateTypeServer,
						DNSNames: []string{"foo.localhost"},
						K8sSecret: domain.K8sSecretConfig{
							Name: "foo-tls",
							Type: domain.K8sSecretTypeTLS,
						},
					},
				},
			},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)

	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	// Keyring fails
	mockKeyring.On("HasKey", "Test-ca-key").Return(false, assert.AnError)

	mockSecretStore := new(testutil.MockSecretStore)
	provisioner := core.NewCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)

	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.NewDevProxyConfigGenerator()
	devProxyManager := core.NewDevProxyManager(
		configRepository, fileSystem, containerImageRepository, containerOrchestrator, configGenerator,
	)
	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewInstallCommandHandler(
		configRepository, containerImageRepository, containerOrchestrator,
		devProxyManager, environmentEnsurer, scm, provisioner,
	)

	result := sut.Handle([]string{}, "all", false, domain.HelmChartOverrides{})

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "failed to provision certificates")
	// Should not proceed to install services
	containerOrchestrator.AssertNotCalled(t, "InstallService", mock.Anything)
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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", assert.AnError)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	scm := new(testutil.MockScm)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "default", false, domain.HelmChartOverrides{})

	assert.Error(t, result)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 0)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 0)
}

func TestInstallCommandHandler_Handle_InstallDevProxyError(t *testing.T) {
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
	configGenerator := core.NewDevProxyConfigGenerator()
	expectedChecksum := configGenerator.GenerateChecksum(configContext, false)

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	// Checksum matches — no rebuild, but InstallDevProxy still called and fails
	containerOrchestrator.On("GetDevProxyChecksum").Return(expectedChecksum, nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(assert.AnError)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("FileExists", "~/.pilot/Test/dev-proxy/helm").Return(true, nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		new(testutil.MockScm),
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "default", false, domain.HelmChartOverrides{})

	assert.Error(t, result)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 0)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 0)
}

func TestInstallCommandHandler_Handle_ScmDownloadError(t *testing.T) {
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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(assert.AnError)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", false, domain.HelmChartOverrides{})

	assert.ErrorIs(t, result, assert.AnError)
	containerOrchestrator.AssertNotCalled(t, "InstallService", mock.Anything, mock.Anything)
}

func TestInstallCommandHandler_Handle_InstallServiceError(t *testing.T) {
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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(assert.AnError)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].HelmRepoPath,
		configContext.Services[0].HelmBranch,
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", false, domain.HelmChartOverrides{})

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "failed to install service")
	scm.AssertExpectations(t)
}

func TestInstallCommandHandler_HandleSkipsScmDownloadForOverriddenService(t *testing.T) {
	localChartDirectory := t.TempDir()
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:                  "service-1",
				HelmRepoPath:          "any-repo-1",
				HelmBranch:            "any-branch-1",
				HelmChartRelativePath: "charts/api",
				HelmPath:              "/cache/service-1",
				Profiles:              []string{"all"},
			},
			{
				Name:         "service-2",
				HelmRepoPath: "any-repo-2",
				HelmBranch:   "any-branch-2",
				HelmPath:     "/cache/service-2",
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(service *domain.Service) bool {
		return service.Name == "service-1" &&
			service.HelmPath == localChartDirectory &&
			service.HelmChartRelativePath == ""
	}), mock.Anything).Return(nil).Once()
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(service *domain.Service) bool {
		return service.Name == "service-2" &&
			service.HelmPath != "" &&
			service.HelmChartRelativePath == ""
	}), mock.Anything).Return(nil).Once()
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		"any-repo-2",
		"any-branch-2",
		mock.Anything,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	overrides := domain.NewHelmChartOverrides(map[string]string{"service-1": localChartDirectory})
	result := sut.Handle([]string{}, "all", false, overrides)

	assert.Nil(t, result)
	scm.AssertNumberOfCalls(t, "Download", 1)
	containerOrchestrator.AssertExpectations(t)
}

func TestInstallCommandHandler_HandleFailsOnUnknownOverrideService(t *testing.T) {
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
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("HomeDir").Return("/home/test", nil)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		noCertProvisioner(),
	)

	overrides := domain.NewHelmChartOverrides(map[string]string{"unknown": "/does-not-exist/local-chart"})
	result := sut.Handle([]string{}, "all", false, overrides)

	require.Error(t, result)
	assert.Contains(t, result.Error(), "unknown")
	assert.Contains(t, result.Error(), "service-1", "error must list the available services")
	scm.AssertNumberOfCalls(t, "Download", 0)
	containerOrchestrator.AssertNotCalled(t, "InstallService", mock.Anything, mock.Anything)
}

func TestInstallCommandHandler_HandleIgnoresOverrideForOutOfScopeService(t *testing.T) {
	localChartDirectory := t.TempDir()
	configContext := &domain.ConfigurationContext{
		Name: "Test",
		Services: []domain.Service{
			{
				Name:         "service-1",
				HelmRepoPath: "any-repo-1",
				HelmBranch:   "any-branch-1",
				HelmPath:     "/cache/service-1",
				Profiles:     []string{"all"},
			},
			{
				Name:         "service-2",
				HelmRepoPath: "any-repo-2",
				HelmBranch:   "any-branch-2",
				HelmPath:     "/cache/service-2",
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	mockRawConfig(configRepository, configContext)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.MatchedBy(func(service *domain.Service) bool {
		return service.Name == "service-1"
	}), mock.Anything).Return(nil).Once()
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On("Download", "any-repo-1", "any-branch-1", mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
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
	sut := NewInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		internalTLSProvisioner("Test"),
	)

	overrides := domain.NewHelmChartOverrides(map[string]string{"service-2": localChartDirectory})
	result := sut.Handle([]string{"service-1"}, "all", false, overrides)

	assert.Nil(t, result)
	scm.AssertNumberOfCalls(t, "Download", 1)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 1)
}
