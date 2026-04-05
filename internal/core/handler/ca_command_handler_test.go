package handler

import (
	"testing"
	"time"

	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/testutil"

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
	return core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)
}

// failingEnvironmentEnsurer returns an EnvironmentEnsurer that will fail with a key mismatch.
func failingEnvironmentEnsurer(configRepository *testutil.MockConfigRepository, configContext *domain.ConfigurationContext) core.EnvironmentEnsurer {
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("current-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil).Maybe()
	configRepository.On("LoadEnvKey", configContext.Name).Return("different-key", nil).Maybe()
	return core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)
}

func TestCACommandHandler_HandlePrint_Success(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"), nil)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.ProvideEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
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

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.ProvideEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
	)

	err := sut.HandlePrint()
	assert.Error(t, err)
}

func TestCACommandHandler_HandleReissue_Success(t *testing.T) {
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

	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	// Reissue checks that a CA exists before proceeding
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)

	// Set up the provisioner's passphrase flow
	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("test-passphrase", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", mock.Anything, domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleReissue()
	assert.NoError(t, err)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCACommandHandler_HandleReissue_NoCAExists(t *testing.T) {
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

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleReissue()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificate authority exists")
	assert.Contains(t, err.Error(), "dx ca recreate")
}

func TestCACommandHandler_HandleReissue_NoUserCertsStillReissuesInternalTLS(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name:     "test-ctx",
		Services: []domain.Service{{Name: "svc"}},
	}

	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)

	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	// Reissue checks that a CA exists before proceeding
	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)

	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("pass", nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", core.InternalTLSSecretName, domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleReissue()
	assert.NoError(t, err)

	mockOrch.AssertExpectations(t)
}

func TestCACommandHandler_HandleRecreate_NonTTYWithoutYes(t *testing.T) {
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

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleRecreate(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires confirmation. Use --yes")
}

func TestCACommandHandler_HandleRecreate_UserCancels(t *testing.T) {
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

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleRecreate(false)
	assert.NoError(t, err) // cancelled, not an error
}

func TestCACommandHandler_HandleRecreate_InteractiveYes(t *testing.T) {
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
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)
	mockCA.On("DeleteCA", "test-ctx").Return(nil)
	mockKeyring.On("DeleteKey", "test-ctx-ca-key").Return(nil)

	// ReissueCertificates flow
	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-pass"), nil)
	mockKeyring.On("SetKey", "test-ctx-ca-key", "new-pass").Return(nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("new-pass", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", mock.Anything, domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockTerminal := new(testutil.MockTerminalInput)
	mockTerminal.On("IsTerminal").Return(true)
	mockTerminal.On("ReadLine", mock.Anything).Return("y", nil)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleRecreate(false)
	assert.NoError(t, err)

	mockCA.AssertCalled(t, "DeleteCA", "test-ctx")
	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCACommandHandler_HandleReissue_EnvironmentEnsurer_LoadConfigError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, assert.AnError)

	environmentEnsurer := core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := ProvideCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleReissue()
	assert.Error(t, err)
}

func TestCACommandHandler_HandleRecreate_SkipConfirmation(t *testing.T) {
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
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	mockCA.On("GetCACertificatePEM", "test-ctx").Return([]byte("cert"), nil)
	mockCA.On("DeleteCA", "test-ctx").Return(nil)
	mockKeyring.On("DeleteKey", "test-ctx-ca-key").Return(nil)

	// ReissueCertificates flow
	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-pass"), nil)
	mockKeyring.On("SetKey", "test-ctx-ca-key", "new-pass").Return(nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("new-pass", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", mock.Anything, domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockTerminal := new(testutil.MockTerminalInput)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleRecreate(true)
	assert.NoError(t, err)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
	// ReadLine should never be called with skipConfirmation=true
	mockTerminal.AssertNotCalled(t, "ReadLine", mock.Anything)
}

func TestCACommandHandler_HandleStatus_NoCA(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificateExpiry", "test-ctx").Return(nil, assert.AnError)

	sut := ProvideCACommandHandler(
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
	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)
	mockOrch.On("SecretExists", core.InternalTLSSecretName).Return(false, nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, nil, nil)

	sut := ProvideCACommandHandler(
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
	mockOrch.On("SecretExists", core.InternalTLSSecretName).Return(false, nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, nil, nil)

	sut := ProvideCACommandHandler(
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
	mockOrch.On("SecretExists", core.InternalTLSSecretName).Return(false, nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, nil, nil)

	sut := ProvideCACommandHandler(
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

func TestCACommandHandler_HandleRecreate_NoExistingCA(t *testing.T) {
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
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	// No CA exists
	mockCA.On("GetCACertificatePEM", "test-ctx").Return(nil, assert.AnError)

	// Should proceed without confirmation and without deleting
	mockKeyring.On("HasKey", "test-ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-pass"), nil)
	mockKeyring.On("SetKey", "test-ctx-ca-key", "new-pass").Return(nil)
	mockKeyring.On("GetKey", "test-ctx-ca-key").Return("new-pass", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", mock.Anything, domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	provisioner := core.ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)
	mockTerminal := new(testutil.MockTerminalInput)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		provisioner,
		mockTerminal,
		environmentEnsurer,
	)

	err := sut.HandleRecreate(false)
	assert.NoError(t, err)

	// Should never prompt for confirmation
	mockTerminal.AssertNotCalled(t, "ReadLine", mock.Anything)
	// Should never delete CA
	mockCA.AssertNotCalled(t, "DeleteCA", mock.Anything)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCACommandHandler_HandlePrint_LoadContextNameError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("", assert.AnError)

	sut := ProvideCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.ProvideEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
	)

	err := sut.HandlePrint()
	assert.Error(t, err)
	configRepository.AssertExpectations(t)
}

func TestCACommandHandler_HandleRecreate_EnvironmentEnsurer_LoadConfigError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	containerOrchestrator := new(testutil.MockContainerOrchestrator)
	containerOrchestrator.On("CreateClusterEnvironmentKey").Return("any-key", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, assert.AnError)

	environmentEnsurer := core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := ProvideCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleRecreate(false)
	assert.Error(t, err)
	configRepository.AssertExpectations(t)
}

func TestCACommandHandler_HandleStatus_LoadContextNameError(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := passingEnvironmentEnsurer(configRepository, configContext)
	configRepository.On("LoadCurrentContextName").Return("", assert.AnError)

	sut := ProvideCACommandHandler(
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

	environmentEnsurer := core.ProvideEnvironmentEnsurer(configRepository, containerOrchestrator)

	sut := ProvideCACommandHandler(
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

func TestCACommandHandler_HandlePrint_NoCA_UserFriendlyError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentContextName").Return("test-ctx", nil)

	mockCA := new(testutil.MockCertificateAuthority)
	mockCA.On("GetCACertificatePEM", "test-ctx").Return(nil, assert.AnError)

	sut := ProvideCACommandHandler(
		configRepository,
		mockCA,
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		core.ProvideEnvironmentEnsurer(configRepository, new(testutil.MockContainerOrchestrator)),
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

	sut := ProvideCACommandHandler(
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

func TestCACommandHandler_HandleReissue_EnvironmentMismatch(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := failingEnvironmentEnsurer(configRepository, configContext)

	sut := ProvideCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleReissue()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "environment key mismatch")
}

func TestCACommandHandler_HandleRecreate_EnvironmentMismatch(t *testing.T) {
	configContext := &domain.ConfigurationContext{Name: "test-ctx"}
	configRepository := new(testutil.MockConfigRepository)
	environmentEnsurer := failingEnvironmentEnsurer(configRepository, configContext)

	sut := ProvideCACommandHandler(
		configRepository,
		new(testutil.MockCertificateAuthority),
		noCertProvisioner(),
		new(testutil.MockTerminalInput),
		environmentEnsurer,
	)

	err := sut.HandleRecreate(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "environment key mismatch")
}
