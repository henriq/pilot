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

	"dx/internal/core/domain"
	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testCertPEM generates a self-signed cert PEM that expires at the given time.
func testCertPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	return testCertPEMWithSANs(t, notAfter, nil)
}

// testCertPEMWithSANs generates a self-signed cert PEM with specific DNS SANs and expiry.
func testCertPEMWithSANs(t *testing.T, notAfter time.Time, dnsNames []string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		DNSNames:     dnsNames,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func TestCertificateProvisioner_ProvisionCertificates_NoCertificates(t *testing.T) {
	sut := ProvideCertificateProvisioner(nil, nil, nil, nil)

	secrets, err := sut.ProvisionCertificates(nil, "ctx")
	assert.NoError(t, err)
	assert.Empty(t, secrets)
}

func TestCertificateProvisioner_ProvisionCertificates_SkipsExistingSecrets(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Return a cert with matching DNS names and 20 days remaining (above the 14-day threshold)
	validCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"foo.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": validCert,
		"tls.key": []byte("key"),
		"ca.crt":  []byte("ca"),
	}, nil)

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

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Empty(t, secrets)

	mockOrch.AssertNotCalled(t, "CreateOrUpdateSecret", mock.Anything, mock.Anything, mock.Anything)
	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_IssuesAndCreatesSecret(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)

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

	expectedData := map[string][]byte{
		"tls.crt": []byte("cert-pem"),
		"tls.key": []byte("key-pem"),
		"ca.crt":  []byte("ca-pem"),
	}
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, expectedData).Return(nil)

	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_OpaqueSecretData(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", "bar-tls").Return(false, nil)

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

	expectedData := map[string][]byte{
		"my-cert": []byte("cert-pem"),
		"my-key":  []byte("key-pem"),
		"my-ca":   []byte("ca-pem"),
	}
	mockOrch.On("CreateOrUpdateSecret", "bar-tls", domain.K8sSecretTypeOpaque, expectedData).Return(nil)

	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar-tls"}, secrets)

	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_CreatesPassphraseIfMissing(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return([]byte("new-passphrase"), nil)
	mockKeyring.On("SetKey", "ctx-ca-key", "new-passphrase").Return(nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("new-passphrase", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Return a cert with 20 days remaining (above the 14-day threshold) and matching SANs
	validCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"foo.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": validCert,
	}, nil)

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

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)

	mockEncryptor.AssertExpectations(t)
	mockKeyring.AssertExpectations(t)
}

func TestCertificateProvisioner_ReissueCertificates(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

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

	expectedData := map[string][]byte{
		"tls.crt": []byte("cert-pem"),
		"tls.key": []byte("key-pem"),
		"ca.crt":  []byte("ca-pem"),
	}
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, expectedData).Return(nil)

	services := []domain.ServiceCertificates{
		{
			Name:         "svc",
			Certificates: []domain.CertificateRequest{certReq},
		},
	}

	secrets, err := sut.ReissueCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)

	// Reissue should NOT check SecretExists (forces overwrite)
	mockOrch.AssertNotCalled(t, "SecretExists", mock.Anything)
	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_IssueCertificateCAError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)
	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("failed to load or create CA: %w", assert.AnError))

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestCertificateProvisioner_ProvisionCertificates_SecretExistsError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(false, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check secret")
}

func TestCertificateProvisioner_ProvisionCertificates_IssueCertificateError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestCertificateProvisioner_ProvisionCertificates_CreateSecretError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, mock.Anything).Return(assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create secret")
}

func TestCertificateProvisioner_ProvisionCertificates_KeyringHasKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(nil, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check keyring for key")
}

func TestCertificateProvisioner_ProvisionCertificates_CreateKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(nil, nil, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(false, nil)
	mockEncryptor.On("CreateKey").Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create encryption key")
}

func TestCertificateProvisioner_ProvisionCertificates_SetKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(nil, nil, mockKeyring, mockEncryptor)

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

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store encryption key")
}

func TestCertificateProvisioner_ProvisionCertificates_GetKeyError(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(nil, nil, mockKeyring, mockEncryptor)

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

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve encryption key")
}

func TestCertificateProvisioner_ProvisionCertificates_MultipleCertificates(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", mock.Anything).Return(false, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil)

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

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls", "bar-tls"}, secrets)
}

func TestCertificateProvisioner_ReissueCertificates_IssueCertificateCAError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, nil, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{
		{Name: "svc", Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		}},
	}

	_, err := sut.ReissueCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestCertificateProvisioner_ReissueCertificates_IssueCertificateError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{
		{Name: "svc", Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		}},
	}

	_, err := sut.ReissueCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestCertificateProvisioner_ReissueCertificates_CreateSecretError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, mock.Anything).Return(assert.AnError)

	services := []domain.ServiceCertificates{
		{Name: "svc", Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		}},
	}

	_, err := sut.ReissueCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create secret")
}

func TestCertificateProvisioner_DeletePassphrase(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(nil, nil, mockKeyring, nil)

	mockKeyring.On("DeleteKey", "ctx-ca-key").Return(nil)

	err := sut.DeletePassphrase("ctx")
	require.NoError(t, err)

	mockKeyring.AssertExpectations(t)
}

func TestCertificateProvisioner_DeletePassphrase_Error(t *testing.T) {
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(nil, nil, mockKeyring, nil)

	mockKeyring.On("DeleteKey", "ctx-ca-key").Return(assert.AnError)

	err := sut.DeletePassphrase("ctx")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestCertificateProvisioner_ProvisionCertificates_RenewsExpiringCert(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Cert expires in 10 days — below the 14-day threshold (SANs match so only expiry triggers renewal)
	expiringCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 10), []string{"foo.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": expiringCert,
	}, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("new-cert"), KeyPEM: []byte("new-key"), CAPEM: []byte("ca")}
	certReq := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"foo.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "foo-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, certReq).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, map[string][]byte{
		"tls.crt": []byte("new-cert"),
		"tls.key": []byte("new-key"),
		"ca.crt":  []byte("ca"),
	}).Return(nil)

	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_RenewsWhenCertKeyMissing(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Secret exists but has no tls.crt key
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.key": []byte("key"),
	}, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)
}

func TestCertificateProvisioner_ProvisionCertificates_RenewsWhenPEMInvalid(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Secret has garbage data that can't be PEM-decoded
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": []byte("not-valid-pem"),
	}, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)
}

func TestCertificateProvisioner_ProvisionCertificates_GetSecretDataError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)
	mockOrch.On("GetSecretData", "foo-tls").Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS}},
		},
	}}

	_, err := sut.ProvisionCertificates(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check certificate expiry")
}

func TestCertificateProvisioner_ProvisionCertificates_RenewsOpaqueExpiringCert(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "bar-tls").Return(true, nil)

	// Opaque secret with cert expiring in 5 days (SANs match so only expiry triggers renewal)
	expiringCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 5), []string{"bar.localhost"})
	mockOrch.On("GetSecretData", "bar-tls").Return(map[string][]byte{
		"my-cert": expiringCert,
	}, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("c"), KeyPEM: []byte("k"), CAPEM: []byte("a")}
	certReq := domain.CertificateRequest{
		Type:     domain.CertificateTypeClient,
		DNSNames: []string{"bar.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "bar-tls",
			Type: domain.K8sSecretTypeOpaque,
			Keys: &domain.OpaqueSecretKeys{PrivateKey: "my-key", Cert: "my-cert", CA: "my-ca"},
		},
	}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, certReq).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "bar-tls", domain.K8sSecretTypeOpaque, map[string][]byte{
		"my-cert": []byte("c"),
		"my-key":  []byte("k"),
		"my-ca":   []byte("a"),
	}).Return(nil)

	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar-tls"}, secrets)

	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_ReissuesWhenDNSNamesAdded(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Existing cert has only ["foo.localhost"], but config now has ["foo.localhost", "*.foo.localhost"]
	existingCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"foo.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": existingCert,
	}, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("new-cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"foo.localhost", "*.foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "foo-tls",
					Type: domain.K8sSecretTypeTLS,
				},
			},
		},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_ReissuesWhenDNSNameRemoved(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Existing cert has ["foo.localhost", "bar.localhost"], config now only has ["foo.localhost"]
	existingCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"foo.localhost", "bar.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": existingCert,
	}, nil)

	issued := &domain.IssuedCertificate{CertPEM: []byte("new-cert"), KeyPEM: []byte("key"), CAPEM: []byte("ca")}
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(issued, nil)
	mockOrch.On("CreateOrUpdateSecret", "foo-tls", domain.K8sSecretTypeTLS, mock.Anything).Return(nil)

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

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, secrets)

	mockCA.AssertExpectations(t)
	mockOrch.AssertExpectations(t)
}

func TestCertificateProvisioner_ProvisionCertificates_SkipsWhenDNSNamesMatch(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, nil)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("pass", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	// Existing cert SANs match config (order-independent)
	existingCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"bar.localhost", "foo.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": existingCert,
	}, nil)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"foo.localhost", "bar.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "foo-tls",
					Type: domain.K8sSecretTypeTLS,
				},
			},
		},
	}}

	secrets, err := sut.ProvisionCertificates(services, "ctx")
	assert.NoError(t, err)
	assert.Empty(t, secrets)

	mockOrch.AssertNotCalled(t, "CreateOrUpdateSecret", mock.Anything, mock.Anything, mock.Anything)
}

func TestCertificateProvisioner_GetCertificateStatuses(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := ProvideCertificateProvisioner(nil, mockOrch, nil, nil)

	// foo-tls exists with a valid cert (20 days remaining)
	validCert := testCertPEM(t, time.Now().AddDate(0, 0, 20))
	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{
		"tls.crt": validCert,
	}, nil)

	// bar-tls does not exist
	mockOrch.On("SecretExists", "bar-tls").Return(false, nil)

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

func TestCertificateProvisioner_GetCertificateStatuses_SecretExistsError(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := ProvideCertificateProvisioner(nil, mockOrch, nil, nil)

	mockOrch.On("SecretExists", "foo-tls").Return(false, assert.AnError)

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

func TestCertificateProvisioner_GetCertificateStatuses_GetSecretDataError(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := ProvideCertificateProvisioner(nil, mockOrch, nil, nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)
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

	// GetSecretData errors are swallowed — status returned with Found=true but DaysRemaining=0
	statuses, err := sut.GetCertificateStatuses(services)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.True(t, statuses[0].Found)
	assert.Equal(t, 0, statuses[0].DaysRemaining)
}

func TestCertificateProvisioner_GetCertificateStatuses_MultipleServices(t *testing.T) {
	mockOrch := new(testutil.MockSecretStore)

	sut := ProvideCertificateProvisioner(nil, mockOrch, nil, nil)

	mockOrch.On("SecretExists", "svc-a-tls").Return(false, nil)
	mockOrch.On("SecretExists", "svc-b-tls").Return(false, nil)

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

func TestDnsNamesMatch(t *testing.T) {
	tests := []struct {
		name       string
		configured []string
		actual     []string
		expected   bool
	}{
		{"both empty", nil, nil, true},
		{"both empty slices", []string{}, []string{}, true},
		{"single match", []string{"a.localhost"}, []string{"a.localhost"}, true},
		{"order independent", []string{"b.localhost", "a.localhost"}, []string{"a.localhost", "b.localhost"}, true},
		{"different lengths", []string{"a.localhost"}, []string{"a.localhost", "b.localhost"}, false},
		{"added name", []string{"a.localhost", "b.localhost"}, []string{"a.localhost"}, false},
		{"different names", []string{"a.localhost"}, []string{"b.localhost"}, false},
		{"duplicates in configured", []string{"a.localhost", "a.localhost"}, []string{"a.localhost", "b.localhost"}, false},
		{"nil vs empty", nil, []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dnsNamesMatch(tt.configured, tt.actual)
			assert.Equal(t, tt.expected, result)
		})
	}
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
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)

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

	certsByService, provisioned, err := sut.ProvisionCertificateData(services, "ctx")

	assert.NoError(t, err)
	assert.Equal(t, []string{"foo-tls"}, provisioned)
	require.Len(t, certsByService["svc"], 1)
	assert.Equal(t, certReq, certsByService["svc"][0].Request)
	assert.Equal(t, []byte("cert-pem"), certsByService["svc"][0].Data["tls.crt"])
	assert.Equal(t, []byte("key-pem"), certsByService["svc"][0].Data["tls.key"])
	assert.Equal(t, []byte("ca-pem"), certsByService["svc"][0].Data["ca.crt"])

	mockOrch.AssertNotCalled(t, "CreateOrUpdateSecret", mock.Anything, mock.Anything, mock.Anything)
}

func TestCertificateProvisioner_ProvisionCertificateData_ReusesExistingCert(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)

	validCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"foo.localhost"})
	existingData := map[string][]byte{
		"tls.crt": validCert,
		"tls.key": []byte("existing-key"),
		"ca.crt":  []byte("existing-ca"),
	}
	mockOrch.On("GetSecretData", "foo-tls").Return(existingData, nil)

	certReq := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"foo.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "foo-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}
	services := []domain.ServiceCertificates{{
		Name:         "svc",
		Certificates: []domain.CertificateRequest{certReq},
	}}

	certsByService, provisioned, err := sut.ProvisionCertificateData(services, "ctx")

	assert.NoError(t, err)
	assert.Empty(t, provisioned)
	require.Len(t, certsByService["svc"], 1)
	assert.Equal(t, existingData, certsByService["svc"][0].Data)

	mockCA.AssertNotCalled(t, "IssueCertificate", mock.Anything, mock.Anything, mock.Anything)
	mockOrch.AssertNotCalled(t, "CreateOrUpdateSecret", mock.Anything, mock.Anything, mock.Anything)
}

func TestCertificateProvisioner_ProvisionCertificateData_GroupsByService(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", mock.Anything).Return(false, nil)

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

	certsByService, provisioned, err := sut.ProvisionCertificateData(services, "ctx")

	assert.NoError(t, err)
	assert.Len(t, provisioned, 2)
	assert.Len(t, certsByService, 2)
	require.Len(t, certsByService["svc-a"], 1)
	require.Len(t, certsByService["svc-b"], 1)
	assert.Equal(t, "a-tls", certsByService["svc-a"][0].Request.K8sSecret.Name)
	assert.Equal(t, "b-tls", certsByService["svc-b"][0].Request.K8sSecret.Name)
}

func TestCertificateProvisioner_ProvisionCertificateData_EmptyServices(t *testing.T) {
	sut := ProvideCertificateProvisioner(
		new(testutil.MockCertificateAuthority),
		new(testutil.MockSecretStore),
		new(testutil.MockKeyring),
		new(testutil.MockSymmetricEncryptor),
	)

	certsByService, provisioned, err := sut.ProvisionCertificateData(nil, "ctx")

	assert.NoError(t, err)
	assert.Nil(t, certsByService)
	assert.Nil(t, provisioned)
}

func TestCertificateProvisioner_ProvisionCertificateData_SecretExistsError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)
	mockOrch.On("SecretExists", "foo-tls").Return(false, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{{
			Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
			K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS},
		}},
	}}

	_, _, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check secret foo-tls")
}

func TestCertificateProvisioner_ProvisionCertificateData_IssueCertificateError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)
	mockOrch.On("SecretExists", "foo-tls").Return(false, nil)
	mockCA.On("IssueCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{{
			Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
			K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS},
		}},
	}}

	_, _, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue certificate for foo-tls")
}

func TestCertificateProvisioner_ProvisionCertificateData_GetSecretDataError(t *testing.T) {
	mockCA := new(testutil.MockCertificateAuthority)
	mockOrch := new(testutil.MockSecretStore)
	mockKeyring := new(testutil.MockKeyring)
	mockEncryptor := new(testutil.MockSymmetricEncryptor)

	sut := ProvideCertificateProvisioner(mockCA, mockOrch, mockKeyring, mockEncryptor)

	mockKeyring.On("HasKey", "ctx-ca-key").Return(true, nil)
	mockKeyring.On("GetKey", "ctx-ca-key").Return("test-passphrase", nil)

	mockOrch.On("SecretExists", "foo-tls").Return(true, nil)
	// Return a valid cert so needsRenewal returns false, then GetSecretData fails on the reuse path
	validCert := testCertPEMWithSANs(t, time.Now().AddDate(0, 0, 20), []string{"foo.localhost"})
	mockOrch.On("GetSecretData", "foo-tls").Return(map[string][]byte{"tls.crt": validCert}, nil).Once()
	mockOrch.On("GetSecretData", "foo-tls").Return(nil, assert.AnError).Once()

	services := []domain.ServiceCertificates{{
		Name: "svc",
		Certificates: []domain.CertificateRequest{{
			Type: domain.CertificateTypeServer, DNSNames: []string{"foo.localhost"},
			K8sSecret: domain.K8sSecretConfig{Name: "foo-tls", Type: domain.K8sSecretTypeTLS},
		}},
	}}

	_, _, err := sut.ProvisionCertificateData(services, "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read secret foo-tls")
}
