package certificate_authority

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	"dx/internal/core/domain"
	"dx/internal/ports"
)

var _ ports.CertificateAuthority = (*X509CertificateAuthority)(nil)

const (
	caValidityYears  = 10
	certValidityDays = 30
	caCertFilename   = "ca.crt"
	caKeyFilename    = "ca.key"
	caDirectoryName  = "ca"
)

type X509CertificateAuthority struct {
	fileSystem ports.FileSystem
	encryptor  ports.SymmetricEncryptor

	// cached CA state, scoped to cachedContext
	cachedContext string
	caCert        *x509.Certificate
	caKey         *ecdsa.PrivateKey
	caPEM         []byte
}

func ProvideX509CertificateAuthority(
	fileSystem ports.FileSystem,
	encryptor ports.SymmetricEncryptor,
) *X509CertificateAuthority {
	return &X509CertificateAuthority{
		fileSystem: fileSystem,
		encryptor:  encryptor,
	}
}

func caDir(contextName string) string {
	return filepath.Join("~", ".dx", contextName, caDirectoryName)
}

func caCertPath(contextName string) string {
	return filepath.Join(caDir(contextName), caCertFilename)
}

func caKeyPath(contextName string) string {
	return filepath.Join(caDir(contextName), caKeyFilename)
}

func (ca *X509CertificateAuthority) loadOrCreateCA(contextName string, passphrase string) error {
	exists, err := ca.fileSystem.FileExists(caCertPath(contextName))
	if err != nil {
		return fmt.Errorf("failed to check CA certificate: %w", err)
	}
	if exists {
		return ca.loadCA(contextName, passphrase)
	}
	return ca.createCA(contextName, passphrase)
}

func (ca *X509CertificateAuthority) createCA(contextName string, passphrase string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	serialNumber, err := generateSerialNumber()
	if err != nil {
		return err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("DX CA (%s)", contextName),
			Organization: []string{"DX"},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(caValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal CA private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	encryptedKey, err := ca.encryptor.Encrypt(keyPEM, []byte(passphrase))
	if err != nil {
		return fmt.Errorf("failed to encrypt CA private key: %w", err)
	}

	if err := ca.fileSystem.MkdirAll(caDir(contextName), ports.ReadWriteExecute); err != nil {
		return fmt.Errorf("failed to create CA directory: %w", err)
	}

	if err := ca.fileSystem.WriteFile(caCertPath(contextName), certPEM, ports.ReadWrite); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	if err := ca.fileSystem.WriteFile(caKeyPath(contextName), encryptedKey, ports.ReadWrite); err != nil {
		return fmt.Errorf("failed to write CA private key: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	ca.cachedContext = contextName
	ca.caCert = cert
	ca.caKey = key
	ca.caPEM = certPEM

	return nil
}

func (ca *X509CertificateAuthority) loadCA(contextName string, passphrase string) error {
	certPEM, err := ca.fileSystem.ReadFile(caCertPath(contextName))
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf(
			"CA certificate expired on %s; run 'dx ca recreate' to create a new CA",
			cert.NotAfter.Format("2006-01-02"),
		)
	}

	encryptedKey, err := ca.fileSystem.ReadFile(caKeyPath(contextName))
	if err != nil {
		return fmt.Errorf("failed to read CA private key: %w", err)
	}

	keyPEM, err := ca.encryptor.Decrypt(encryptedKey, []byte(passphrase))
	if err != nil {
		return fmt.Errorf("failed to decrypt CA private key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA private key PEM")
	}

	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key: %w", err)
	}

	ca.cachedContext = contextName
	ca.caCert = cert
	ca.caKey = key
	ca.caPEM = certPEM

	return nil
}

func (ca *X509CertificateAuthority) IssueCertificate(contextName string, passphrase string, request domain.CertificateRequest) (*domain.IssuedCertificate, error) {
	if ca.caCert == nil || ca.caKey == nil || ca.cachedContext != contextName {
		if err := ca.loadOrCreateCA(contextName, passphrase); err != nil {
			return nil, fmt.Errorf("failed to load or create CA: %w", err)
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate key: %w", err)
	}

	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: request.DNSNames[0],
		},
		DNSNames:  request.DNSNames,
		NotBefore: now,
		NotAfter:  now.AddDate(0, 0, certValidityDays),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	switch request.Type {
	case domain.CertificateTypeServer:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	case domain.CertificateTypeClient:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, &key.PublicKey, ca.caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal certificate private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &domain.IssuedCertificate{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		CAPEM:   ca.caPEM,
	}, nil
}

func (ca *X509CertificateAuthority) GetCACertificatePEM(contextName string) ([]byte, error) {
	exists, err := ca.fileSystem.FileExists(caCertPath(contextName))
	if err != nil {
		return nil, fmt.Errorf("failed to check CA certificate: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("no certificate authority found for context '%s'", contextName)
	}

	certPEM, err := ca.fileSystem.ReadFile(caCertPath(contextName))
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	return certPEM, nil
}

func (ca *X509CertificateAuthority) GetCACertificateExpiry(contextName string) (*time.Time, error) {
	certPEM, err := ca.GetCACertificatePEM(contextName)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode CA certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return &cert.NotAfter, nil
}

func (ca *X509CertificateAuthority) DeleteCA(contextName string) error {
	ca.cachedContext = ""
	ca.caCert = nil
	ca.caKey = nil
	ca.caPEM = nil
	return ca.fileSystem.RemoveAll(caDir(contextName))
}

func generateSerialNumber() (*big.Int, error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	return serialNumber, nil
}
