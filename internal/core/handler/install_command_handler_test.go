package handler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// noCertProvisioner creates a CertificateProvisioner with unconfigured mocks.
// Only safe for test paths that never invoke provisioning (e.g., error paths
// that return before reaching certificate provisioning).
func noCertProvisioner() *core.CertificateProvisioner {
	return core.ProvideCertificateProvisioner(
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

	mockSecretStore.On("GetSecretData", core.InternalTLSSecretName).Return(map[string][]byte(nil), nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	return core.ProvideCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)
}

// generateTestCertPEM creates a self-signed cert PEM with 20 days remaining and the given DNS names.
func generateTestCertPEM(t *testing.T, dnsNames []string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(0, 0, 20),
		DNSNames:     dnsNames,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

// existingCertProvisioner creates a CertificateProvisioner where the internal TLS
// certificate already exists and does not need renewal. No new certs are provisioned.
func existingCertProvisioner(t *testing.T, contextName string, certPEM []byte) *core.CertificateProvisioner {
	t.Helper()

	mockCA := new(testutil.MockCertificateAuthority)
	mockSecretStore := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	keyName := contextName + "-ca-key"
	mockKeyring.On("HasKey", keyName).Return(true, nil)
	mockKeyring.On("GetKey", keyName).Return("test-pass", nil)

	mockSecretStore.On("GetSecretData", core.InternalTLSSecretName).Return(map[string][]byte{
		"tls.crt": certPEM,
		"tls.key": []byte("key"),
		"ca.crt":  []byte("ca"),
	}, nil)

	return core.ProvideCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)
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
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
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
		internalTLSProvisioner("Test"),
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
		internalTLSProvisioner("Test"),
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
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	// No GetDevProxyChecksum mock needed — interceptHttp always triggers rebuild
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
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", true)

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
	configGenerator := core.ProvideDevProxyConfigGenerator()

	// Build the same cert secrets that the provisioner will return, so the checksum matches.
	validCertPEM := generateTestCertPEM(t, core.InternalTLSDNSNames(configContext))
	internalTLSReq := core.InternalTLSCertificateRequest(configContext)
	devProxyCertSecrets, err := core.RenderCertificateSecretManifests([]domain.ProvisionedCertificate{{
		Request: *internalTLSReq,
		Data:    map[string][]byte{"tls.crt": validCertPEM, "tls.key": []byte("key"), "ca.crt": []byte("ca")},
	}})
	require.NoError(t, err)

	expectedChecksum := configGenerator.GenerateChecksum(configContext, false, devProxyCertSecrets)

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	// Return matching checksum - dev-proxy should be skipped
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
	// Use a provisioner where the internal TLS cert already exists and is valid,
	// so no new certs are provisioned and the dev-proxy install is skipped.
	sut := ProvideInstallCommandHandler(
		configRepository,
		containerImageRepository,
		containerOrchestrator,
		devProxyManager,
		environmentEnsurer,
		scm,
		existingCertProvisioner(t, "Test", validCertPEM),
	)

	result := sut.Handle([]string{}, "default", false)

	assert.Nil(t, result)
	// Verify BuildImage was NOT called since dev-proxy checksum matched
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 0)
	// Verify InstallDevProxy was NOT called (no config change, no new certs)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 0)
	// Verify WriteFile was NOT called for dev-proxy config
	fileSystem.AssertNumberOfCalls(t, "WriteFile", 0)
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
	configGenerator := core.ProvideDevProxyConfigGenerator()

	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadEnvKey", mock.Anything).Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
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
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)

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
		internalTLSProvisioner("Test"),
	)

	// Even though checksum would match, dev-proxy should always rebuild with interceptHttp
	result := sut.Handle([]string{}, "default", true)

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
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	containerOrchestrator.On("InstallService", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("InstallDevProxy", mock.Anything, mock.Anything).Return(nil)
	containerOrchestrator.On("GetDevProxyChecksum").Return("", nil)

	mockSecretStore := new(testutil.MockSecretStore)
	mockSecretStore.On("GetSecretData", "foo-tls").Return(map[string][]byte(nil), nil)
	mockSecretStore.On("GetSecretData", core.InternalTLSSecretName).Return(map[string][]byte(nil), nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	mockKeyring.On("HasKey", "Test-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "Test-ca-key").Return("test-pass", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)

	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fileSystem.On("HomeDir").Return("/home/test", nil)
	fileSystem.On("RemoveAll", mock.Anything).Return(nil)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	devProxyManager := core.ProvideDevProxyManager(
		configRepository, fileSystem, containerImageRepository, containerOrchestrator, configGenerator,
	)
	environmentEnsurer := core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := ProvideInstallCommandHandler(
		configRepository, containerImageRepository, containerOrchestrator,
		devProxyManager, environmentEnsurer, scm, provisioner,
	)

	result := sut.Handle([]string{}, "all", false)

	assert.Nil(t, result)
	mockSecretStore.AssertCalled(t, "GetSecretData", "foo-tls")
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
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	// Keyring fails
	mockKeyring.On("HasKey", "Test-ca-key").Return(false, assert.AnError)

	mockSecretStore := new(testutil.MockSecretStore)
	provisioner := core.ProvideCertificateProvisioner(mockCA, mockSecretStore, mockKeyring, mockEncryptor)

	fileSystem := new(testutil.MockFileSystem)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	configGenerator := core.ProvideDevProxyConfigGenerator()
	devProxyManager := core.ProvideDevProxyManager(
		configRepository, fileSystem, containerImageRepository, containerOrchestrator, configGenerator,
	)
	environmentEnsurer := core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := ProvideInstallCommandHandler(
		configRepository, containerImageRepository, containerOrchestrator,
		devProxyManager, environmentEnsurer, scm, provisioner,
	)

	result := sut.Handle([]string{}, "all", false)

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
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "default", false)

	assert.Error(t, result)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallService", 0)
	containerOrchestrator.AssertNumberOfCalls(t, "InstallDevProxy", 0)
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
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
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
		configContext.Services[0].HelmPath,
	).Return(assert.AnError)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
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
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", false)

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
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
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
		configContext.Services[0].HelmPath,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything).Return(nil)
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
		internalTLSProvisioner("Test"),
	)

	result := sut.Handle([]string{}, "all", false)

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "failed to install service")
	scm.AssertExpectations(t)
}
