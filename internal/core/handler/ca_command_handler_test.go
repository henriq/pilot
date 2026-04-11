package handler

import (
	"testing"
	"time"

	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// passingEnvironmentEnsurer sets up mock expectations so that EnsureExpectedClusterIsSelected passes,
// then returns the EnvironmentEnsurer. The configRepository mock must already have
// LoadCurrentConfigurationContext configured if the handler also calls it.
func passingEnvironmentEnsurer(
	configRepository *testutil.MockConfigRepository,
	configContext *domain.ConfigurationContext,
) core.EnvironmentEnsurer {
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("matching-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil).Maybe()
	configRepository.On("LoadEnvKey", configContext.Name).Return("matching-key", nil).Maybe()
	return core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)
}

// failingEnvironmentEnsurer returns an EnvironmentEnsurer that will fail with a key mismatch.
func failingEnvironmentEnsurer(configRepository *testutil.MockConfigRepository, configContext *domain.ConfigurationContext) core.EnvironmentEnsurer {
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("current-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil).Maybe()
	configRepository.On("LoadEnvKey", configContext.Name).Return("different-key", nil).Maybe()
	return core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)
}

func TestCACommandHandler_HandlePrint_Success(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"), nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.NewEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
	)

	err := sut.HandlePrint()
	assert.NoError(t, err)

	configRepository.AssertExpectations(t)
	mockCA.AssertExpectations(t)
}

func TestCACommandHandler_HandlePrint_NoCA(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return(nil, assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.NewEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
	)

	err := sut.HandlePrint()
	assert.Error(t, err)
}

func TestCACommandHandler_HandleDelete_NonTTYWithoutYes(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)

	mockTerminal := new(testutil.MockTerminalInput)
	mockTerminal.On("IsTerminal").Return(false)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleDelete(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deleting the CA requires confirmation")
}

func TestCACommandHandler_HandleDelete_UserCancels(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)

	mockTerminal := new(testutil.MockTerminalInput)
	mockTerminal.On("IsTerminal").Return(true)
	mockTerminal.On("ReadLine", mock.Anything).Return("n", nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleDelete(false)
	assert.NoError(t, err) // cancelled, not an error
}

func TestCACommandHandler_HandleDelete_InteractiveYes(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)
	mockCA.On("DeleteCA", "test-ctx").Return(nil)
	mockKeyring.On("DeleteKey", "test-ctx-ca-key").Return(nil)

	provisioner := core.NewCertificateProvisioner(mockCA, new(testutil.MockSecretStore), mockKeyring, nil)

	mockTerminal := new(testutil.MockTerminalInput)
	mockTerminal.On("IsTerminal").Return(true)
	mockTerminal.On("ReadLine", mock.Anything).Return("y", nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleDelete(false)
	assert.NoError(t, err)

	mockCA.AssertCalled(t, "DeleteCA", "test-ctx")
	mockCA.AssertExpectations(t)
}

func TestCACommandHandler_HandleDelete_SkipConfirmation(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)
	mockCA.On("DeleteCA", "test-ctx").Return(nil)
	mockKeyring.On("DeleteKey", "test-ctx-ca-key").Return(nil)

	provisioner := core.NewCertificateProvisioner(mockCA, new(testutil.MockSecretStore), mockKeyring, nil)

	mockTerminal := new(testutil.MockTerminalInput)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleDelete(true)
	assert.NoError(t, err)

	mockCA.AssertExpectations(t)
	// ReadLine should never be called with skipConfirmation=true
	mockTerminal.AssertNotCalled(t, "ReadLine", mock.Anything)
}

func TestCACommandHandler_HandleDelete_DeleteCAError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)
	mockCA.On("DeleteCA", "test-ctx").Return(assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleDelete(true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove existing CA")
}

func TestCACommandHandler_HandleDelete_DeletePassphraseError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)
	mockCA.On("DeleteCA", "test-ctx").Return(nil)
	mockKeyring.On("DeleteKey", "test-ctx-ca-key").Return(assert.AnError)

	provisioner := core.NewCertificateProvisioner(mockCA, new(testutil.MockSecretStore), mockKeyring, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleDelete(true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove CA passphrase")
}

func TestCACommandHandler_HandleStatus_NoCA(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificateExpiry", "test-ctx").Return(nil, assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.NoError(t, err) // no CA is info, not error
}

func TestCACommandHandler_HandleStatus_ValidCA(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{
				Name: "svc",
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
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	expiry := time.Now().AddDate(10, 0, 0)
	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificateExpiry", "test-ctx").Return(&expiry, nil)

	mockOrch := new(testutil.MockSecretStore)
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte(nil), nil)
	mockOrch.On("GetSecretData", core.InternalTLSSecretName).Return(map[string][]byte(nil), nil)

	provisioner := core.NewCertificateProvisioner(mockCA, mockOrch, nil, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.NoError(t, err)

	mockCA.AssertExpectations(t)
}

func TestCACommandHandler_HandleStatus_ExpiredCA(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name:     "test-ctx",
		Services: []domain.Service{},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	expiry := time.Now().AddDate(0, 0, -5)
	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificateExpiry", "test-ctx").Return(&expiry, nil)

	mockOrch := new(testutil.MockSecretStore)
	mockOrch.On("GetSecretData", core.InternalTLSSecretName).Return(map[string][]byte(nil), nil)

	provisioner := core.NewCertificateProvisioner(mockCA, mockOrch, nil, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.NoError(t, err)

	mockCA.AssertExpectations(t)
}

func TestCACommandHandler_HandleStatus_ExpiringSoonCA(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name:     "test-ctx",
		Services: []domain.Service{},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	expiry := time.Now().AddDate(0, 0, 15)
	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificateExpiry", "test-ctx").Return(&expiry, nil)

	mockOrch := new(testutil.MockSecretStore)
	mockOrch.On("GetSecretData", core.InternalTLSSecretName).Return(map[string][]byte(nil), nil)

	provisioner := core.NewCertificateProvisioner(mockCA, mockOrch, nil, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.NoError(t, err)

	mockCA.AssertExpectations(t)
}

func TestCACommandHandler_HandleDelete_NoExistingCA(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-ctx",
		Services: []domain.Service{
			{Name: "svc", Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			}},
		},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return(nil, assert.AnError)

	mockTerminal := new(testutil.MockTerminalInput)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleDelete(false)
	assert.NoError(t, err)

	// Should never prompt for confirmation or delete CA
	mockTerminal.AssertNotCalled(t, "ReadLine", mock.Anything)
	mockCA.AssertNotCalled(t, "DeleteCA", mock.Anything)
}

func TestCACommandHandler_HandlePrint_LoadContextNameError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("", assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.NewEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
	)

	err := sut.HandlePrint()
	assert.Error(t, err)
	configRepository.AssertExpectations(t)
}

func TestCACommandHandler_HandleDelete_EnvironmentEnsurer_LoadConfigError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, assert.AnError)

	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleDelete(false)
	assert.Error(t, err)
	configRepository.AssertExpectations(t)
}

func TestCACommandHandler_HandleStatus_LoadContextNameError(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("", assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.Error(t, err)
	configRepository.AssertExpectations(t)
}

func TestCACommandHandler_HandleStatus_EnvironmentEnsurer_LoadConfigContextError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, assert.AnError)

	environmentEnsurer := core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.Error(t, err)
	configRepository.AssertExpectations(t)
}

func TestCACommandHandler_HandleIssue_ServerCert(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("test-passphrase", nil)

	issuedCert := &domain.IssuedCertificate{
		CertPEM: []byte("cert-pem"),
		KeyPEM:  []byte("key-pem"),
		CAPEM:   []byte("ca-pem"),
	}
	mockCA.On("IssueCertificate", "test-ctx", "test-passphrase", mock.MatchedBy(func(req domain.CertificateRequest) bool {
		return req.Type == domain.CertificateTypeServer && len(req.DNSNames) == 1 && req.DNSNames[0] == "myapp.test"
	})).Return(issuedCert, nil)

	provisioner := core.NewCertificateProvisioner(mockCA, new(testutil.MockSecretStore), mockKeyring, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	contextName, issued, err := sut.HandleIssue("server", []string{"myapp.test"})
	assert.NoError(t, err)
	assert.Equal(t, "test-ctx", contextName)
	assert.Equal(t, []byte("cert-pem"), issued.CertPEM)
	assert.Equal(t, []byte("key-pem"), issued.KeyPEM)
	assert.Equal(t, []byte("ca-pem"), issued.CAPEM)

	mockCA.AssertExpectations(t)
	mockKeyring.AssertExpectations(t)
}

func TestCACommandHandler_HandleIssue_ClientCert(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("test-passphrase", nil)

	issuedCert := &domain.IssuedCertificate{
		CertPEM: []byte("client-cert"),
		KeyPEM:  []byte("client-key"),
		CAPEM:   []byte("ca-cert"),
	}
	mockCA.On("IssueCertificate", "test-ctx", "test-passphrase", mock.MatchedBy(func(req domain.CertificateRequest) bool {
		return req.Type == domain.CertificateTypeClient && len(req.DNSNames) == 1 && req.DNSNames[0] == "api.localhost"
	})).Return(issuedCert, nil)

	provisioner := core.NewCertificateProvisioner(mockCA, new(testutil.MockSecretStore), mockKeyring, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	contextName, issued, err := sut.HandleIssue("client", []string{"api.localhost"})
	assert.NoError(t, err)
	assert.Equal(t, "test-ctx", contextName)
	assert.Equal(t, []byte("client-cert"), issued.CertPEM)
	assert.Equal(t, []byte("client-key"), issued.KeyPEM)
	assert.Equal(t, []byte("ca-cert"), issued.CAPEM)

	mockCA.AssertExpectations(t)
	mockKeyring.AssertExpectations(t)
}

func TestCACommandHandler_HandleIssue_InvalidType(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("invalid", []string{"foo.test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid certificate type 'invalid'")
}

func TestCACommandHandler_HandleIssue_InvalidDNSName(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("server", []string{"api.example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-reserved TLD")
}

func TestCACommandHandler_HandleIssue_EmptyDNSNames(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("server", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty dnsNames")
}

func TestCACommandHandler_HandleIssue_LoadContextNameError(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("", assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("server", []string{"foo.test"})
	assert.Error(t, err)
}

func TestCACommandHandler_HandleIssue_PassphraseError(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockKeyring := new(testutil.MockKeyring)
	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(false, assert.AnError)

	provisioner := core.NewCertificateProvisioner(
		new(testutil.MockCertificateAuthority),
		new(testutil.MockSecretStore),
		mockKeyring,
		new(testutil.MockSymmetricEncryptor),
	)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("server", []string{"foo.test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve CA passphrase")
}

func TestCACommandHandler_HandleIssue_IssueCertificateError(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("test-passphrase", nil)
	mockCA.On("IssueCertificate", "test-ctx", "test-passphrase", mock.Anything).Return(nil, assert.AnError)

	provisioner := core.NewCertificateProvisioner(mockCA, new(testutil.MockSecretStore), mockKeyring, nil)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("server", []string{"foo.test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestCACommandHandler_HandleIssue_EnvironmentMismatch(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := failingEnvironmentEnsurer(configRepository, configContext)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	_, _, err := sut.HandleIssue("server", []string{"foo.test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "environment key mismatch")
}

func TestCACommandHandler_HandlePrint_NoCA_UserFriendlyError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return(nil, assert.AnError)

	sut := NewCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.NewEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
	)

	err := sut.HandlePrint()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificate authority exists")
	assert.Contains(t, err.Error(), "test-ctx")
}

func TestCACommandHandler_HandleStatus_EnvironmentMismatch(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := failingEnvironmentEnsurer(configRepository, configContext)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleStatus()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "environment key mismatch")
}

func TestCACommandHandler_HandleDelete_EnvironmentMismatch(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := failingEnvironmentEnsurer(configRepository, configContext)

	sut := NewCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleDelete(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "environment key mismatch")
}
