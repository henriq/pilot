package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// CertificateType determines the x509 key usage extensions for an issued certificate.
type CertificateType string

const (
	CertificateTypeServer CertificateType = "server"
	CertificateTypeClient CertificateType = "client"
)

// K8sSecretType controls the Kubernetes secret type used to store the certificate.
type K8sSecretType string

const (
	K8sSecretTypeTLS    K8sSecretType = "tls"
	K8sSecretTypeOpaque K8sSecretType = "opaque"
)

// CertificateRequest defines a certificate to be issued and stored as a Kubernetes secret.
type CertificateRequest struct {
	Type      CertificateType `yaml:"type"`
	DNSNames  []string        `yaml:"dnsNames"`
	K8sSecret K8sSecretConfig `yaml:"k8sSecret"`
}

// K8sSecretConfig defines how the issued certificate is stored in Kubernetes.
type K8sSecretConfig struct {
	Name string            `yaml:"name"`
	Type K8sSecretType     `yaml:"type"`
	Keys *OpaqueSecretKeys `yaml:"keys,omitempty"`
}

// OpaqueSecretKeys defines custom key names for opaque Kubernetes secrets.
type OpaqueSecretKeys struct {
	PrivateKey string `yaml:"privateKey"`
	Cert       string `yaml:"cert"`
	CA         string `yaml:"ca"`
}

// IssuedCertificate holds a newly issued certificate and its private key in PEM format.
type IssuedCertificate struct {
	CertPEM []byte
	KeyPEM  []byte
	CAPEM   []byte
}

// ServiceCertificates pairs a service name with its certificate requests.
// Used by CertificateProvisioner to decouple certificate provisioning from
// the full Service type.
type ServiceCertificates struct {
	Name         string
	Certificates []CertificateRequest
}

// ProvisionedCertificate pairs a certificate request with its secret data,
// ready to be rendered as a K8s Secret manifest in a Helm wrapper chart.
type ProvisionedCertificate struct {
	Request CertificateRequest
	Data    map[string][]byte
}

var dnsNameRegex = regexp.MustCompile(`^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)
var k8sNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`)
var k8sSecretKeyRegex = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)

// allowedDNSSuffixes restricts certificate DNS names to reserved/special-use TLDs
// that cannot collide with real public domains. This prevents the private CA from
// being misused to issue certificates for public domains when added to a trust store.
//
// Sources:
//   - .localhost  — RFC 6761 (loopback)
//   - .test       — RFC 2606 (testing)
//   - .example    — RFC 2606 (documentation)
//   - .invalid    — RFC 2606 (guaranteed non-resolvable)
//   - .local      — RFC 6762 (mDNS; also covers K8s internal names like foo.svc.cluster.local)
//   - .internal   — ICANN (2024, private/internal use)
//   - .home.arpa  — RFC 8375 (home networks)
var allowedDNSSuffixes = []string{
	".localhost",
	".test",
	".example",
	".invalid",
	".local",
	".internal",
	".home.arpa",
}

// hasAllowedDNSSuffix checks whether a DNS name (without wildcard prefix) ends
// with one of the allowed reserved TLDs, or is exactly a bare reserved TLD.
func hasAllowedDNSSuffix(name string) bool {
	name = strings.TrimPrefix(name, "*.")
	lower := strings.ToLower(name)
	for _, suffix := range allowedDNSSuffixes {
		if lower == suffix[1:] || strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// labelsWithinLimit checks that every label in a DNS name is at most 63 characters (RFC 1035).
func labelsWithinLimit(name string) bool {
	name = strings.TrimPrefix(name, "*.")
	for _, label := range strings.Split(name, ".") {
		if len(label) > 63 {
			return false
		}
	}
	return true
}

// Validate checks the certificate request for correctness.
func (c *CertificateRequest) Validate(serviceName, contextName string) error {
	if c.Type != CertificateTypeServer && c.Type != CertificateTypeClient {
		return fmt.Errorf(
			"certificate for service '%s' in context '%s' has invalid type '%s' (must be 'server' or 'client')",
			serviceName, contextName, c.Type,
		)
	}

	if len(c.DNSNames) == 0 {
		return fmt.Errorf(
			"certificate for service '%s' in context '%s' has empty dnsNames",
			serviceName, contextName,
		)
	}
	for i, name := range c.DNSNames {
		if name == "" {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has empty dnsNames entry at index %d",
				serviceName, contextName, i,
			)
		}
		if len(name) > 253 {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has dnsNames entry '%s' exceeding 253 characters",
				serviceName, contextName, name,
			)
		}
		if !dnsNameRegex.MatchString(name) {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has invalid dnsNames entry '%s'",
				serviceName, contextName, name,
			)
		}
		if !labelsWithinLimit(name) {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has dnsNames entry '%s' with a label exceeding 63 characters",
				serviceName, contextName, name,
			)
		}
		if !hasAllowedDNSSuffix(name) {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has dnsNames entry '%s' with a non-reserved TLD; "+
					"only reserved TLDs are allowed (.localhost, .test, .example, .invalid, .local, .internal, .home.arpa)",
				serviceName, contextName, name,
			)
		}
	}

	if c.K8sSecret.Name == "" {
		return fmt.Errorf(
			"certificate for service '%s' in context '%s' has empty k8sSecret.name",
			serviceName, contextName,
		)
	}
	if len(c.K8sSecret.Name) > 253 || !k8sNameRegex.MatchString(c.K8sSecret.Name) {
		return fmt.Errorf(
			"certificate for service '%s' in context '%s' has invalid k8sSecret.name '%s' (must be a valid Kubernetes name: lowercase alphanumeric, hyphens, or dots)",
			serviceName, contextName, c.K8sSecret.Name,
		)
	}

	if c.K8sSecret.Type != K8sSecretTypeTLS && c.K8sSecret.Type != K8sSecretTypeOpaque {
		return fmt.Errorf(
			"certificate for service '%s' in context '%s' has invalid k8sSecret.type '%s' (must be 'tls' or 'opaque')",
			serviceName, contextName, c.K8sSecret.Type,
		)
	}

	if c.K8sSecret.Type == K8sSecretTypeTLS && c.K8sSecret.Keys != nil {
		return fmt.Errorf(
			"certificate for service '%s' in context '%s' must not specify keys when k8sSecret.type is 'tls'",
			serviceName, contextName,
		)
	}

	if c.K8sSecret.Type == K8sSecretTypeOpaque {
		if c.K8sSecret.Keys == nil {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' must specify keys when k8sSecret.type is 'opaque'",
				serviceName, contextName,
			)
		}
		if c.K8sSecret.Keys.PrivateKey == "" {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has empty k8sSecret.keys.privateKey",
				serviceName, contextName,
			)
		}
		if !k8sSecretKeyRegex.MatchString(c.K8sSecret.Keys.PrivateKey) {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has invalid k8sSecret.keys.privateKey '%s' (must match [-._a-zA-Z0-9]+)",
				serviceName, contextName, c.K8sSecret.Keys.PrivateKey,
			)
		}
		if c.K8sSecret.Keys.Cert == "" {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has empty k8sSecret.keys.cert",
				serviceName, contextName,
			)
		}
		if !k8sSecretKeyRegex.MatchString(c.K8sSecret.Keys.Cert) {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has invalid k8sSecret.keys.cert '%s' (must match [-._a-zA-Z0-9]+)",
				serviceName, contextName, c.K8sSecret.Keys.Cert,
			)
		}
		if c.K8sSecret.Keys.CA == "" {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has empty k8sSecret.keys.ca",
				serviceName, contextName,
			)
		}
		if !k8sSecretKeyRegex.MatchString(c.K8sSecret.Keys.CA) {
			return fmt.Errorf(
				"certificate for service '%s' in context '%s' has invalid k8sSecret.keys.ca '%s' (must match [-._a-zA-Z0-9]+)",
				serviceName, contextName, c.K8sSecret.Keys.CA,
			)
		}
	}

	return nil
}
