package core

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

// InternalTLSSecretName is the K8s secret name for the dev-proxy TLS certificate.
const InternalTLSSecretName = "pilot-internal-tls" //nolint:gosec // Not a credential, just a K8s secret resource name

// CertificateStatus holds the status of a provisioned certificate.
type CertificateStatus struct {
	ServiceName   string
	SecretName    string
	CertType      domain.CertificateType
	DNSNames      []string
	Found         bool
	DaysRemaining int
}

// CertificateProvisioner issues certificates and provides certificate status information.
type CertificateProvisioner struct {
	ca          ports.CertificateAuthority
	secretStore ports.SecretStore
	keyring     ports.Keyring
	encryptor   ports.SymmetricEncryptor
}

func NewCertificateProvisioner(
	ca ports.CertificateAuthority,
	secretStore ports.SecretStore,
	keyring ports.Keyring,
	encryptor ports.SymmetricEncryptor,
) *CertificateProvisioner {
	return &CertificateProvisioner{
		ca:          ca,
		secretStore: secretStore,
		keyring:     keyring,
		encryptor:   encryptor,
	}
}

func (p *CertificateProvisioner) caKeyName(contextName string) string {
	return fmt.Sprintf("%s-ca-key", contextName)
}

// InternalTLSDNSNames returns the DNS names for the dev-proxy internal TLS certificate.
// These must match the hosts used in the dev-proxy ingress template.
func InternalTLSDNSNames(configContext *domain.ConfigurationContext) []string {
	names := []string{
		fmt.Sprintf("dev-proxy.%s.localhost", configContext.Name),
		fmt.Sprintf("stats.dev-proxy.%s.localhost", configContext.Name),
	}
	for _, ls := range configContext.LocalServices {
		names = append(names, fmt.Sprintf("%s.%s.localhost", ls.Name, configContext.Name))
	}
	return names
}

// InternalTLSCertificateRequest builds a CertificateRequest for the dev-proxy ingresses.
func InternalTLSCertificateRequest(configContext *domain.ConfigurationContext) *domain.CertificateRequest {
	return &domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: InternalTLSDNSNames(configContext),
		K8sSecret: domain.K8sSecretConfig{
			Name: InternalTLSSecretName,
			Type: domain.K8sSecretTypeTLS,
		},
	}
}

// CollectAllCertificates extracts certificate requests from the given services
// and appends the internal TLS certificate for the dev-proxy.
// Services without certificates are omitted from the result.
func CollectAllCertificates(
	services []domain.Service,
	configContext *domain.ConfigurationContext,
) []domain.ServiceCertificates {
	var result []domain.ServiceCertificates
	for _, svc := range services {
		if len(svc.Certificates) > 0 {
			result = append(result, domain.ServiceCertificates{
				Name:         svc.Name,
				Certificates: svc.Certificates,
			})
		}
	}
	internalCertReq := InternalTLSCertificateRequest(configContext)
	return append(result, domain.ServiceCertificates{
		Name:         "dev-proxy",
		Certificates: []domain.CertificateRequest{*internalCertReq},
	})
}

// ProvisionCertificateData issues certificates for all services via the CA.
// Returns all certificate data grouped by service name for inclusion in Helm wrapper charts.
func (p *CertificateProvisioner) ProvisionCertificateData(
	services []domain.ServiceCertificates,
	contextName string,
) (map[string][]domain.ProvisionedCertificate, error) {
	if len(services) == 0 {
		return nil, nil
	}

	passphrase, err := p.GetOrCreatePassphrase(contextName)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]domain.ProvisionedCertificate)

	for _, svc := range services {
		for _, certReq := range svc.Certificates {
			issued, err := p.ca.IssueCertificate(contextName, passphrase, certReq)
			if err != nil {
				return nil, fmt.Errorf("failed to issue certificate for %s: %w", certReq.K8sSecret.Name, err)
			}

			data, err := buildSecretData(certReq, issued)
			if err != nil {
				return nil, fmt.Errorf("failed to build secret data for %s: %w", certReq.K8sSecret.Name, err)
			}

			result[svc.Name] = append(result[svc.Name], domain.ProvisionedCertificate{
				Request: certReq,
				Data:    data,
			})
		}
	}

	return result, nil
}

// GetCertificateStatuses returns the status of each certificate across all services.
func (p *CertificateProvisioner) GetCertificateStatuses(services []domain.ServiceCertificates) ([]CertificateStatus, error) {
	var statuses []CertificateStatus
	for _, service := range services {
		for _, certReq := range service.Certificates {
			status := CertificateStatus{
				ServiceName: service.Name,
				SecretName:  certReq.K8sSecret.Name,
				CertType:    certReq.Type,
				DNSNames:    certReq.DNSNames,
			}

			data, err := p.secretStore.GetSecretData(certReq.K8sSecret.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to check secret %s: %w", certReq.K8sSecret.Name, err)
			}
			if data == nil {
				statuses = append(statuses, status)
				continue
			}
			status.Found = true

			certKey := certPEMKey(certReq)
			certPEM, ok := data[certKey]
			if !ok || len(certPEM) == 0 {
				statuses = append(statuses, status)
				continue
			}

			block, _ := pem.Decode(certPEM)
			if block == nil {
				statuses = append(statuses, status)
				continue
			}

			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				statuses = append(statuses, status)
				continue
			}

			status.DaysRemaining = int(time.Until(cert.NotAfter).Hours() / 24)
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}

// GetOrCreatePassphrase retrieves or creates the CA encryption passphrase for a context.
func (p *CertificateProvisioner) GetOrCreatePassphrase(contextName string) (string, error) {
	return GetOrCreateEncryptionKey(p.keyring, p.encryptor, p.caKeyName(contextName))
}

// DeletePassphrase removes the CA encryption passphrase from the keyring.
func (p *CertificateProvisioner) DeletePassphrase(contextName string) error {
	return p.keyring.DeleteKey(p.caKeyName(contextName))
}

func certPEMKey(req domain.CertificateRequest) string {
	switch req.K8sSecret.Type {
	case domain.K8sSecretTypeTLS:
		return "tls.crt"
	case domain.K8sSecretTypeOpaque:
		return req.K8sSecret.Keys.Cert
	default:
		return ""
	}
}

func buildSecretData(req domain.CertificateRequest, issued *domain.IssuedCertificate) (map[string][]byte, error) {
	switch req.K8sSecret.Type {
	case domain.K8sSecretTypeTLS:
		return map[string][]byte{
			"tls.crt": issued.CertPEM,
			"tls.key": issued.KeyPEM,
			"ca.crt":  issued.CAPEM,
		}, nil
	case domain.K8sSecretTypeOpaque:
		keys := req.K8sSecret.Keys
		return map[string][]byte{
			keys.Cert:       issued.CertPEM,
			keys.PrivateKey: issued.KeyPEM,
			keys.CA:         issued.CAPEM,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported k8sSecret.type '%s'", req.K8sSecret.Type)
	}
}
