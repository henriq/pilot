package ports

import (
	"time"

	"pilot/internal/core/domain"
)

// CertificateAuthority manages a private CA and issues certificates.
type CertificateAuthority interface {
	// IssueCertificate issues a new certificate signed by the CA for the given context.
	// The CA is loaded (or created if none exists) automatically using the provided passphrase.
	IssueCertificate(contextName string, passphrase string, request domain.CertificateRequest) (*domain.IssuedCertificate, error)

	// GetCACertificatePEM returns the PEM-encoded CA certificate.
	GetCACertificatePEM(contextName string) ([]byte, error)

	// GetCACertificateExpiry returns the expiration time of the CA certificate.
	GetCACertificateExpiry(contextName string) (*time.Time, error)

	// DeleteCA removes the CA certificate and private key files.
	DeleteCA(contextName string) error
}
