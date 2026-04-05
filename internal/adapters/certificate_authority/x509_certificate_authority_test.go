package certificate_authority

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

	"dx/internal/adapters/symmetric_encryptor"
	"dx/internal/core/domain"
	"dx/internal/ports"
	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestCA(t *testing.T) (*X509CertificateAuthority, *testutil.TestFileSystem, string) {
	t.Helper()
	fs := testutil.NewTestFileSystem(t)
	encryptor := symmetric_encryptor.ProvideAesGcmEncryptor()

	// Generate a real encryption key for tests
	keyBytes, err := encryptor.CreateKey()
	require.NoError(t, err)
	passphrase := string(keyBytes)

	ca := ProvideX509CertificateAuthority(fs, encryptor)
	return ca, fs, passphrase
}

func TestX509CertificateAuthority_CreateCA_GeneratesValidCA(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	assert.NotNil(t, ca.caCert)
	assert.NotNil(t, ca.caKey)
	assert.NotNil(t, ca.caPEM)

	assert.True(t, ca.caCert.IsCA)
	assert.Equal(t, "DX CA (test-ctx)", ca.caCert.Subject.CommonName)
	assert.Contains(t, ca.caCert.Subject.Organization, "DX")
	assert.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign, ca.caCert.KeyUsage)
}

func TestX509CertificateAuthority_CreateCA_ValidityPeriod(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	// CA should be valid for ~10 years
	duration := ca.caCert.NotAfter.Sub(ca.caCert.NotBefore)
	assert.InDelta(t, 10*365.25*24, duration.Hours(), 48) // within 2 days tolerance
}

func TestX509CertificateAuthority_LoadCA_RoundTrip(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	originalCert := ca.caCert.Raw

	// Create a new instance and load the CA
	ca2 := ProvideX509CertificateAuthority(ca.fileSystem, ca.encryptor)
	err = ca2.loadCA("test-ctx", passphrase)
	require.NoError(t, err)

	assert.Equal(t, originalCert, ca2.caCert.Raw)
	assert.True(t, ca2.caCert.IsCA)
}

func TestX509CertificateAuthority_LoadOrCreateCA_CreatesWhenMissing(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.loadOrCreateCA("test-ctx", passphrase)
	require.NoError(t, err)

	assert.NotNil(t, ca.caCert)
	assert.True(t, ca.caCert.IsCA)
}

func TestX509CertificateAuthority_LoadOrCreateCA_LoadsWhenExists(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)
	originalCert := ca.caCert.Raw

	ca2 := ProvideX509CertificateAuthority(ca.fileSystem, ca.encryptor)
	err = ca2.loadOrCreateCA("test-ctx", passphrase)
	require.NoError(t, err)

	assert.Equal(t, originalCert, ca2.caCert.Raw)
}

func TestX509CertificateAuthority_LoadCA_NoExistingCA(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.loadCA("nonexistent-ctx", passphrase)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read CA certificate")
}

func TestX509CertificateAuthority_LoadCA_WrongPassphrase(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	encryptor := symmetric_encryptor.ProvideAesGcmEncryptor()

	correctKey, err := encryptor.CreateKey()
	require.NoError(t, err)
	wrongKey, err := encryptor.CreateKey()
	require.NoError(t, err)

	ca := ProvideX509CertificateAuthority(fs, encryptor)
	err = ca.createCA("test-ctx", string(correctKey))
	require.NoError(t, err)

	ca2 := ProvideX509CertificateAuthority(fs, encryptor)
	err = ca2.loadCA("test-ctx", string(wrongKey))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt CA private key")
}

func TestX509CertificateAuthority_IssueCertificate_ServerCert(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	request := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"foo.localhost", "*.foo.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "foo-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}

	issued, err := ca.IssueCertificate("test-ctx", passphrase, request)
	require.NoError(t, err)
	require.NotNil(t, issued)

	assert.NotEmpty(t, issued.CertPEM)
	assert.NotEmpty(t, issued.KeyPEM)
	assert.NotEmpty(t, issued.CAPEM)

	// Parse and verify the issued certificate
	block, _ := pem.Decode(issued.CertPEM)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Equal(t, "foo.localhost", cert.Subject.CommonName)
	assert.Equal(t, []string{"foo.localhost", "*.foo.localhost"}, cert.DNSNames)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	assert.False(t, cert.IsCA)
}

func TestX509CertificateAuthority_IssueCertificate_ClientCert(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	request := domain.CertificateRequest{
		Type:     domain.CertificateTypeClient,
		DNSNames: []string{"bar.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "bar-tls",
			Type: domain.K8sSecretTypeOpaque,
			Keys: &domain.OpaqueSecretKeys{
				PrivateKey: "key",
				Cert:       "cert",
				CA:         "ca",
			},
		},
	}

	issued, err := ca.IssueCertificate("test-ctx", passphrase, request)
	require.NoError(t, err)

	block, _ := pem.Decode(issued.CertPEM)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
}

func TestX509CertificateAuthority_IssueCertificate_CertSignedByCA(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	request := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"test.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "test-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}

	issued, err := ca.IssueCertificate("test-ctx", passphrase, request)
	require.NoError(t, err)

	// Verify the cert is signed by the CA
	certBlock, _ := pem.Decode(issued.CertPEM)
	require.NotNil(t, certBlock)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	roots.AddCert(ca.caCert)

	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	assert.NoError(t, err)
}

func TestX509CertificateAuthority_IssueCertificate_ValidityPeriod(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	request := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"test.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "test-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}

	issued, err := ca.IssueCertificate("test-ctx", passphrase, request)
	require.NoError(t, err)

	block, _ := pem.Decode(issued.CertPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)

	duration := cert.NotAfter.Sub(cert.NotBefore)
	assert.InDelta(t, 30*24, duration.Hours(), 24) // within 1 day tolerance
}

func TestX509CertificateAuthority_IssueCertificate_CreatesCAOnDemand(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	request := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"test.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "test-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}

	// No CA created yet — IssueCertificate should create it automatically
	issued, err := ca.IssueCertificate("test-ctx", passphrase, request)
	require.NoError(t, err)
	assert.NotEmpty(t, issued.CertPEM)
}

func TestX509CertificateAuthority_GetCACertificatePEM(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	certPEM, err := ca.GetCACertificatePEM("test-ctx")
	require.NoError(t, err)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)
}

func TestX509CertificateAuthority_GetCACertificatePEM_NoCA(t *testing.T) {
	ca, _, _ := setupTestCA(t)

	_, err := ca.GetCACertificatePEM("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificate authority found")
}

func TestX509CertificateAuthority_GetCACertificateExpiry(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	expiry, err := ca.GetCACertificateExpiry("test-ctx")
	require.NoError(t, err)
	require.NotNil(t, expiry)

	assert.Equal(t, ca.caCert.NotAfter, *expiry)
}

func TestX509CertificateAuthority_GetCACertificateExpiry_NoCA(t *testing.T) {
	ca, _, _ := setupTestCA(t)

	_, err := ca.GetCACertificateExpiry("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificate authority found")
}

func TestX509CertificateAuthority_GetCACertificateExpiry_CorruptedCert(t *testing.T) {
	ca, fs, _ := setupTestCA(t)

	require.NoError(t, fs.MkdirAll(caDir("test-ctx"), ports.ReadWriteExecute))
	require.NoError(t, fs.WriteFile(caCertPath("test-ctx"), []byte("not a PEM"), ports.ReadWrite))

	_, err := ca.GetCACertificateExpiry("test-ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode CA certificate PEM")
}

func TestX509CertificateAuthority_IssueCertificate_ContextSwitch(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	// Create CA and issue cert for context A
	err := ca.createCA("ctx-a", passphrase)
	require.NoError(t, err)
	certA := ca.caCert.Raw

	request := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"foo.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "foo-tls",
			Type: domain.K8sSecretTypeTLS,
		},
	}

	issuedA, err := ca.IssueCertificate("ctx-a", passphrase, request)
	require.NoError(t, err)
	assert.NotEmpty(t, issuedA.CertPEM)

	// Issue cert for context B — should create a new CA
	issuedB, err := ca.IssueCertificate("ctx-b", passphrase, request)
	require.NoError(t, err)
	assert.NotEmpty(t, issuedB.CertPEM)

	// The CA should have switched
	assert.Equal(t, "ctx-b", ca.cachedContext)
	assert.NotEqual(t, certA, ca.caCert.Raw)
}

func TestX509CertificateAuthority_DeleteCA(t *testing.T) {
	ca, fs, passphrase := setupTestCA(t)
	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	exists, _ := fs.FileExists(caCertPath("test-ctx"))
	assert.True(t, exists)

	err = ca.DeleteCA("test-ctx")
	require.NoError(t, err)

	exists, _ = fs.FileExists(caCertPath("test-ctx"))
	assert.False(t, exists)
}

func TestX509CertificateAuthority_CreateCA_OverwritesExisting(t *testing.T) {
	ca, _, passphrase := setupTestCA(t)

	err := ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)
	firstCert := ca.caCert.Raw

	err = ca.createCA("test-ctx", passphrase)
	require.NoError(t, err)

	// Should be a different certificate (different serial number and key)
	assert.NotEqual(t, firstCert, ca.caCert.Raw)
}

func TestX509CertificateAuthority_LoadCA_ExpiredCA(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	encryptor := symmetric_encryptor.ProvideAesGcmEncryptor()

	keyBytes, err := encryptor.CreateKey()
	require.NoError(t, err)
	passphrase := string(keyBytes)

	// Capture the expected expiry date before creating the cert to avoid midnight boundary flakiness
	expiredDate := time.Now().AddDate(0, 0, -1)
	expectedDateStr := expiredDate.Format("2006-01-02")

	// Generate an expired CA certificate
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Expired CA"},
		NotBefore:             time.Now().AddDate(0, 0, -30),
		NotAfter:              expiredDate,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	encryptedKey, err := encryptor.Encrypt(keyPEM, []byte(passphrase))
	require.NoError(t, err)

	require.NoError(t, fs.MkdirAll(caDir("test-ctx"), ports.ReadWriteExecute))
	require.NoError(t, fs.WriteFile(caCertPath("test-ctx"), certPEM, ports.ReadWrite))
	require.NoError(t, fs.WriteFile(caKeyPath("test-ctx"), encryptedKey, ports.ReadWrite))

	ca := ProvideX509CertificateAuthority(fs, encryptor)
	err = ca.loadCA("test-ctx", passphrase)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate expired on")
	assert.Contains(t, err.Error(), expectedDateStr)
	assert.Contains(t, err.Error(), "dx ca recreate")
	assert.Nil(t, ca.caCert)
	assert.Nil(t, ca.caKey)
	assert.Nil(t, ca.caPEM)
}

func TestX509CertificateAuthority_LoadOrCreateCA_ExpiredCA(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	encryptor := symmetric_encryptor.ProvideAesGcmEncryptor()

	keyBytes, err := encryptor.CreateKey()
	require.NoError(t, err)
	passphrase := string(keyBytes)

	// Capture expired date before creating cert to avoid midnight boundary flakiness
	expiredDate := time.Now().AddDate(0, 0, -1)

	// Generate an expired CA certificate
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Expired CA"},
		NotBefore:             time.Now().AddDate(0, 0, -30),
		NotAfter:              expiredDate,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	encryptedKey, err := encryptor.Encrypt(keyPEM, []byte(passphrase))
	require.NoError(t, err)

	require.NoError(t, fs.MkdirAll(caDir("test-ctx"), ports.ReadWriteExecute))
	require.NoError(t, fs.WriteFile(caCertPath("test-ctx"), certPEM, ports.ReadWrite))
	require.NoError(t, fs.WriteFile(caKeyPath("test-ctx"), encryptedKey, ports.ReadWrite))

	ca := ProvideX509CertificateAuthority(fs, encryptor)
	err = ca.loadOrCreateCA("test-ctx", passphrase)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate expired on")
	assert.Contains(t, err.Error(), "dx ca recreate")
}
