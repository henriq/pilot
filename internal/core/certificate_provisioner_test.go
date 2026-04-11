package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testCertPEM generates a self-signed cert PEM that expires at the given time.
func testCertPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func TestCertificateProvisioner_ProvisionCertificateData_OpaqueSecretData(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert-pem"),
		KeyPEM:  []byte("key-pem"),
		CAPEM:   []byte("ca-pem"),
	}
	certReq := domain.CertificateRequest{
		Type:     domain.CertificateTypeClient,
		DNSNames: []string{"bar.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "bar-tls",
			Type: domain.K8sSecretTypeOpaque,
			Keys: &domain.OpaqueSecretKeys{
				PrivateKey: "my-key",
				Cert:       "my-cert",
				CA:         "my-ca",
			},
		},
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, certReq).Return(issued, nil)

	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	certsByService, err := sut.ProvisionCertificateData(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []byte("cert-pem"), certsByService["svc"][0].Data["my-cert"])
	assert.Equal(t, []byte("key-pem"), certsByService["svc"][0].Data["my-key"])
	assert.Equal(t, []byte("ca-pem"), certsByService["svc"][0].Data["my-ca"])
}

func TestCertificateProvisioner_ProvisionCertificateData_CreatesPassphraseIfMissing(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-passphrase"), nil)
	mockKeyring.On("SetKey", "ctx-ca-key", "new-passphrase").Return(nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("new-passphrase", nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	services := []domain.ServiceCertificates{{
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
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.NoError(t, err)

	mockEncryptor.AssertExpectations(t)
	mockKeyring.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificateData_IssueCertificateCAError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("failed to load or create CA: %w", assert.AnError))

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestCertificateProvisioner_ProvisionCertificateData_KeyringHasKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(nil, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check keyring for key")
}

func TestCertificateProvisioner_ProvisionCertificateData_CreateKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := NewCertificateProvisioner(nil, nil, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create encryption key")
}

func TestCertificateProvisioner_ProvisionCertificateData_SetKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := NewCertificateProvisioner(nil, nil, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-key"), nil)
	mockKeyring.On("SetKey", "ctx-ca-key", "new-key").Return(assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store encryption key")
}

func TestCertificateProvisioner_ProvisionCertificateData_GetKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := NewCertificateProvisioner(nil, nil, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-key"), nil)
	mockKeyring.On("SetKey", "ctx-ca-key", "new-key").Return(nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("", assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve encryption key")
}

func TestCertificateProvisioner_ProvisionCertificateData_MultipleCertificates(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			{Type: domain.CertificateTypeClient, DNSNames: []string{"bar.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "bar-tls", Type: domain.K8sSecretTypeOpaque,
					Keys: &domain.OpaqueSecretKeys{PrivateKey: "key", Cert: "cert", CA: "ca"}}},
		},
	}}

	certsByService, err := sut.ProvisionCertificateData(services, "ctx")
	assert.NoError(t, err)
	assert.Len(t, certsByService["svc"], 2)
}

func TestCertificateProvisioner_DeletePassphrase(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(nil, nil, mockKeyring, nil)

	mockKeyring.On("DeleteKey", "ctx-ca-key").Return(nil)

	err := sut.DeletePassphrase("ctx")
	require.NoError(t, err)

	mockKeyring.AssertExpectations(t)
}

func TestCertificateProvisioner_DeletePassphrase_Error(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(nil, nil, mockKeyring, nil)

	mockKeyring.On("DeleteKey", "ctx-ca-key").Return(assert.AnError)

	err := sut.DeletePassphrase("ctx")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestCertificateProvisioner_GetCertificateStatuses(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := NewCertificateProvisioner(nil, mockOrch, nil, nil)

	// foo-tls exists with a valid cert (20 days remaining)
	validCert := testCertPEM(t, time.Now().AddDate(0, 0, 20))
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": validCert,
	}, nil)

	// bar-tls does not exist
	mockOrch.On("GetSecretData", "bar-tls").Return(map[string][]byte(nil), nil)

	services := []domain.ServiceCertificates{
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
				{
					Type:     domain.CertificateTypeServer,
					DNSNames: []string{"bar.localhost", "*.bar.localhost"},
					K8sSecret: domain.K8sSecretConfig{
						Name: "bar-tls",
						Type: domain.K8sSecretTypeTLS,
					},
				},
			},
		},
	}

	statuses, err := sut.GetCertificateStatuses(services)
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	assert.Equal(t, "svc", statuses[0].ServiceName)
	assert.Equal(t, "foo-tls", statuses[0].SecretName)
	assert.Equal(t, domain.CertificateTypeServer, statuses[0].CertType)
	assert.Equal(t, []string{"foo.localhost"}, statuses[0].DNSNames)
	assert.True(t, statuses[0].Found)
	assert.InDelta(t, 20, statuses[0].DaysRemaining, 1)

	assert.Equal(t, "svc", statuses[1].ServiceName)
	assert.Equal(t, "bar-tls", statuses[1].SecretName)
	assert.Equal(t, domain.CertificateTypeServer, statuses[1].CertType)
	assert.Equal(t, []string{"bar.localhost", "*.bar.localhost"}, statuses[1].DNSNames)
	assert.False(t, statuses[1].Found)
	assert.Equal(t, 0, statuses[1].DaysRemaining)
}

func TestCertificateProvisioner_GetCertificateStatuses_GetSecretDataError(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := NewCertificateProvisioner(nil, mockOrch, nil, nil)

	mockOrch.On("GetSecretData", "foo-tls").Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{
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
	}

	_, err := sut.GetCertificateStatuses(services)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "foo-tls")
}

func TestCertificateProvisioner_GetCertificateStatuses_MultipleServices(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := NewCertificateProvisioner(nil, mockOrch, nil, nil)

	mockOrch.On("GetSecretData", "svc-a-tls").Return(map[string][]byte(nil), nil)
	mockOrch.On("GetSecretData", "svc-b-tls").Return(map[string][]byte(nil), nil)

	services := []domain.ServiceCertificates{
		{
			Name: "service-a",
			Certificates: []domain.CertificateRequest{
				{
					Type:     domain.CertificateTypeServer,
					DNSNames: []string{"a.localhost"},
					K8sSecret: domain.K8sSecretConfig{
						Name: "svc-a-tls",
						Type: domain.K8sSecretTypeTLS,
					},
				},
			},
		},
		{
			Name: "service-b",
			Certificates: []domain.CertificateRequest{
				{
					Type:     domain.CertificateTypeClient,
					DNSNames: []string{"b.localhost"},
					K8sSecret: domain.K8sSecretConfig{
						Name: "svc-b-tls",
						Type: domain.K8sSecretTypeTLS,
					},
				},
			},
		},
	}

	statuses, err := sut.GetCertificateStatuses(services)
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	assert.Equal(t, "service-a", statuses[0].ServiceName)
	assert.Equal(t, domain.CertificateTypeServer, statuses[0].CertType)

	assert.Equal(t, "service-b", statuses[1].ServiceName)
	assert.Equal(t, domain.CertificateTypeClient, statuses[1].CertType)
}

func TestBuildSecretData_UnsupportedType(t *testing.T) {
	req := domain.CertificateRequest{
		K8sSecret: domain.K8sSecretConfig{
			Type: "unsupported",
		},
	}
	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"),
		KeyPEM:  []byte("key"),
		CAPEM:   []byte("ca"),
	}

	_, err := buildSecretData(req, issued)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported k8sSecret.type")
}

func TestCollectAllCertificates_IncludesServicesWithCerts(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
	}
	services := []domain.Service{
		{
			Name: "svc-with-certs",
			Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
			},
		},
	}

	result := CollectAllCertificates(services, configContext)

	require.Len(t, result, 2) // user service + dev-proxy
	assert.Equal(t, "svc-with-certs", result[0].Name)
	assert.Len(t, result[0].Certificates, 1)
	assert.Equal(t, "dev-proxy", result[1].Name)
}

func TestCollectAllCertificates_SkipsServicesWithoutCerts(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
	}
	services := []domain.Service{
		{Name: "no-certs"},
		{
			Name: "has-certs",
			Certificates: []domain.CertificateRequest{
				{Type: domain.CertificateTypeServer, DNSNames: []string{"bar.localhost"},
					K8sSecret: domain.K8sSecretConfig{Name: "bar-tls", Type: domain.K8sSecretTypeTLS}},
			},
		},
	}

	result := CollectAllCertificates(services, configContext)

	require.Len(t, result, 2) // has-certs + dev-proxy (no-certs skipped)
	assert.Equal(t, "has-certs", result[0].Name)
	assert.Equal(t, "dev-proxy", result[1].Name)
}

func TestCollectAllCertificates_AlwaysAppendsInternalTLS(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		LocalServices: []domain.LocalService{
			{Name: "api"},
		},
	}

	result := CollectAllCertificates(nil, configContext)

	require.Len(t, result, 1)
	assert.Equal(t, "dev-proxy", result[0].Name)
	assert.Equal(t, InternalTLSSecretName, result[0].Certificates[0].K8sSecret.Name)
	assert.Contains(t, result[0].Certificates[0].DNSNames, "dev-proxy.ctx.localhost")
	assert.Contains(t, result[0].Certificates[0].DNSNames, "api.ctx.localhost")
}

func TestCertificateProvisioner_ProvisionCertificateData_IssuesNewCert(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert-pem"),
		KeyPEM:  []byte("key-pem"),
		CAPEM:   []byte("ca-pem"),
	}
	certReq := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"foo.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "foo-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, certReq).Return(issued, nil)

	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	certsByService, err := sut.ProvisionCertificateData(services, "ctx")

	assert.NoError(t, err)
	require.Len(t, certsByService["svc"], 1)
	assert.Equal(t, certReq, certsByService["svc"][0].Request)
	assert.Equal(t, []byte("cert-pem"), certsByService["svc"][0].Data["tls.crt"])
	assert.Equal(t, []byte("key-pem"), certsByService["svc"][0].Data["tls.key"])
	assert.Equal(t, []byte("ca-pem"), certsByService["svc"][0].Data["ca.crt"])
}

func TestCertificateProvisioner_ProvisionCertificateData_GroupsByService(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	issued := &domain.IssuedCertificate{
		CertPEM: []byte("cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca"),
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)

	services := []domain.ServiceCertificates{
		{
			Name: "svc-a",
			Certificates: []domain.CertificateRequest{{
				Type: domain.CertificateTypeServer, DNSNames: []string{"a.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "a-tls", Type: domain.K8sSecretTypeTLS},
			}},
		},
		{
			Name: "svc-b",
			Certificates: []domain.CertificateRequest{{
				Type: domain.CertificateTypeServer, DNSNames: []string{"b.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "b-tls", Type: domain.K8sSecretTypeTLS},
			}},
		},
	}

	certsByService, err := sut.ProvisionCertificateData(services, "ctx")

	assert.NoError(t, err)
	assert.Len(t, certsByService, 2)
	require.Len(t, certsByService["svc-a"], 1)
	require.Len(t, certsByService["svc-b"], 1)
	assert.Equal(t, "a-tls", certsByService["svc-a"][0].Request.K8sSecret.Name)
	assert.Equal(t, "b-tls", certsByService["svc-b"][0].Request.K8sSecret.Name)
}

func TestCertificateProvisioner_ProvisionCertificateData_EmptyServices(t *testing.T) {
	sut := NewCertificateProvisioner(
		new(testutil.MockCertificateAuthority),
		new(testutil.MockSecretStore),
		new(testutil.MockKeyring),
		new(testutil.MockSymmetricEncryptor),
	)

	certsByService, err := sut.ProvisionCertificateData(nil, "ctx")

	assert.NoError(t, err)
	assert.Nil(t, certsByService)
}

func TestCertificateProvisioner_ProvisionCertificateData_IssueCertificateError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := NewCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{{
			Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
			K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS},
		}},
	}}

	_, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate for foo-tls")
}
