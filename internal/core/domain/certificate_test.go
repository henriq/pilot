package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validServerCertificateRequest() CertificateRequest {
	return CertificateRequest{
		Type:     CertificateTypeServer,
		DNSNames: []string{"foo.localhost"},
		K8sSecret: K8sSecretConfig{
			Name: "foo-tls",
			Type: K8sSecretTypeTLS,
		},
	}
}

func validClientCertificateRequest() CertificateRequest {
	return CertificateRequest{
		Type:     CertificateTypeClient,
		DNSNames: []string{"bar.localhost"},
		K8sSecret: K8sSecretConfig{
			Name: "bar-tls",
			Type: K8sSecretTypeOpaque,
			Keys: &OpaqueSecretKeys{
				PrivateKey: "key",
				Cert:       "cert",
				CA:         "ca",
			},
		},
	}
}

func TestCertificateRequest_Validate_ValidServer(t *testing.T) {
	cert := validServerCertificateRequest()
	assert.NoError(t, cert.Validate("svc", "ctx"))
}

func TestCertificateRequest_Validate_ValidClient(t *testing.T) {
	cert := validClientCertificateRequest()
	assert.NoError(t, cert.Validate("svc", "ctx"))
}

func TestCertificateRequest_Validate_InvalidType(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.Type = "invalid"
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type 'invalid'")
}

func TestCertificateRequest_Validate_EmptyDNSNames(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.DNSNames = nil
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty dnsNames")
}

func TestCertificateRequest_Validate_EmptyDNSNameEntry(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.DNSNames = []string{"foo.localhost", ""}
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty dnsNames entry at index 1")
}

func TestCertificateRequest_Validate_DNSNameExceeds253Characters(t *testing.T) {
	cert := validServerCertificateRequest()
	longLabel := ""
	for i := 0; i < 250; i++ {
		longLabel += "a"
	}
	cert.DNSNames = []string{longLabel + ".localhost"}
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeding 253 characters")
}

func TestCertificateRequest_Validate_DNSLabelExceeds63Characters(t *testing.T) {
	cert := validServerCertificateRequest()
	longLabel := strings.Repeat("a", 64)
	cert.DNSNames = []string{longLabel + ".localhost"}
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "label exceeding 63 characters")
}

func TestCertificateRequest_Validate_DNSLabelExactly63Characters(t *testing.T) {
	cert := validServerCertificateRequest()
	label := strings.Repeat("a", 63)
	cert.DNSNames = []string{label + ".localhost"}
	assert.NoError(t, cert.Validate("svc", "ctx"))
}

func TestCertificateRequest_Validate_EmptySecretName(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.K8sSecret.Name = ""
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty k8sSecret.name")
}

func TestCertificateRequest_Validate_InvalidSecretType(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.K8sSecret.Type = "invalid"
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid k8sSecret.type 'invalid'")
}

func TestCertificateRequest_Validate_TLSWithKeys(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.K8sSecret.Keys = &OpaqueSecretKeys{PrivateKey: "key", Cert: "cert", CA: "ca"}
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not specify keys when k8sSecret.type is 'tls'")
}

func TestCertificateRequest_Validate_OpaqueWithoutKeys(t *testing.T) {
	cert := validClientCertificateRequest()
	cert.K8sSecret.Keys = nil
	err := cert.Validate("svc", "ctx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must specify keys when k8sSecret.type is 'opaque'")
}

func TestCertificateRequest_Validate_OpaqueEmptyKeyFields(t *testing.T) {
	tests := []struct {
		name    string
		keys    OpaqueSecretKeys
		wantMsg string
	}{
		{
			"empty privateKey",
			OpaqueSecretKeys{PrivateKey: "", Cert: "cert", CA: "ca"},
			"empty k8sSecret.keys.privateKey",
		},
		{
			"empty cert",
			OpaqueSecretKeys{PrivateKey: "key", Cert: "", CA: "ca"},
			"empty k8sSecret.keys.cert",
		},
		{
			"empty ca",
			OpaqueSecretKeys{PrivateKey: "key", Cert: "cert", CA: ""},
			"empty k8sSecret.keys.ca",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validClientCertificateRequest()
			cert.K8sSecret.Keys = &tt.keys
			err := cert.Validate("svc", "ctx")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestCertificateRequest_Validate_InvalidDNSName(t *testing.T) {
	tests := []struct {
		name    string
		dnsName string
	}{
		{"spaces", "foo bar.localhost"},
		{"trailing hyphen", "foo-.localhost"},
		{"uppercase with special chars", "FOO@.localhost"},
		{"empty label", "foo..localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validServerCertificateRequest()
			cert.DNSNames = []string{tt.dnsName}
			err := cert.Validate("svc", "ctx")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid dnsNames entry")
		})
	}
}

func TestCertificateRequest_Validate_ValidDNSNames(t *testing.T) {
	tests := []struct {
		name    string
		dnsName string
	}{
		{"simple localhost", "foo.localhost"},
		{"wildcard localhost", "*.foo.localhost"},
		{"bare localhost", "localhost"},
		{"with hyphens", "my-service.example"},
		{"test TLD", "api.test"},
		{"nested test", "foo.bar.test"},
		{"example TLD", "docs.example"},
		{"invalid TLD", "nope.invalid"},
		{"local TLD", "foo.local"},
		{"k8s internal", "foo.svc.cluster.local"},
		{"internal TLD", "api.internal"},
		{"home.arpa", "myhost.home.arpa"},
		{"nested home.arpa", "device.lan.home.arpa"},
		{"wildcard test", "*.api.test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validServerCertificateRequest()
			cert.DNSNames = []string{tt.dnsName}
			assert.NoError(t, cert.Validate("svc", "ctx"))
		})
	}
}

func TestCertificateRequest_Validate_PublicTLDsRejected(t *testing.T) {
	tests := []struct {
		name    string
		dnsName string
	}{
		{"com", "api.example.com"},
		{"org", "service.example.org"},
		{"net", "host.example.net"},
		{"io", "app.example.io"},
		{"dev", "my-app.dev"},
		{"wildcard com", "*.api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validServerCertificateRequest()
			cert.DNSNames = []string{tt.dnsName}
			err := cert.Validate("svc", "ctx")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "non-reserved TLD")
		})
	}
}

func TestCertificateRequest_Validate_InvalidK8sSecretName(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
	}{
		{"uppercase", "Foo-TLS"},
		{"spaces", "foo tls"},
		{"special chars", "foo_tls"},
		{"leading hyphen", "-foo-tls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validServerCertificateRequest()
			cert.K8sSecret.Name = tt.secretName
			err := cert.Validate("svc", "ctx")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid k8sSecret.name")
		})
	}
}

func TestCertificateRequest_Validate_MultipleDNSNames(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.DNSNames = []string{"foo.localhost", "*.foo.localhost", "bar.localhost"}
	assert.NoError(t, cert.Validate("svc", "ctx"))
}

func TestCertificateRequest_Validate_ErrorIncludesServiceAndContext(t *testing.T) {
	cert := validServerCertificateRequest()
	cert.Type = "invalid"
	err := cert.Validate("my-service", "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "my-service")
	assert.Contains(t, err.Error(), "production")
}

func TestConfig_Validate_ServiceWithValidCertificates(t *testing.T) {
	config := Config{
		Contexts: []ConfigurationContext{
			{
				Name: "test",
				Services: []Service{
					{
						Name:                  "svc",
						HelmRepoPath:          "/tmp/helm",
						HelmBranch:            "main",
						HelmChartRelativePath: "charts",
						Certificates: []CertificateRequest{
							validServerCertificateRequest(),
							validClientCertificateRequest(),
						},
					},
				},
			},
		},
	}
	assert.NoError(t, config.Validate())
}

func TestCertificateRequest_Validate_InvalidOpaqueSecretKeyNames(t *testing.T) {
	tests := []struct {
		name    string
		keys    OpaqueSecretKeys
		wantMsg string
	}{
		{
			"privateKey with spaces",
			OpaqueSecretKeys{PrivateKey: "my key", Cert: "cert", CA: "ca"},
			"invalid k8sSecret.keys.privateKey",
		},
		{
			"cert with slashes",
			OpaqueSecretKeys{PrivateKey: "key", Cert: "path/cert", CA: "ca"},
			"invalid k8sSecret.keys.cert",
		},
		{
			"ca with special chars",
			OpaqueSecretKeys{PrivateKey: "key", Cert: "cert", CA: "ca@root"},
			"invalid k8sSecret.keys.ca",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validClientCertificateRequest()
			cert.K8sSecret.Keys = &tt.keys
			err := cert.Validate("svc", "ctx")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestCertificateRequest_Validate_ValidOpaqueSecretKeyNames(t *testing.T) {
	tests := []struct {
		name string
		keys OpaqueSecretKeys
	}{
		{"simple", OpaqueSecretKeys{PrivateKey: "key", Cert: "cert", CA: "ca"}},
		{"dotted", OpaqueSecretKeys{PrivateKey: "tls.key", Cert: "tls.crt", CA: "ca.crt"}},
		{"hyphenated", OpaqueSecretKeys{PrivateKey: "client-key", Cert: "client-cert", CA: "ca-cert"}},
		{"underscored", OpaqueSecretKeys{PrivateKey: "client_key", Cert: "client_cert", CA: "ca_cert"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := validClientCertificateRequest()
			cert.K8sSecret.Keys = &tt.keys
			assert.NoError(t, cert.Validate("svc", "ctx"))
		})
	}
}

func TestConfig_Validate_ServiceWithInvalidCertificate(t *testing.T) {
	invalidCert := validServerCertificateRequest()
	invalidCert.K8sSecret.Name = ""

	config := Config{
		Contexts: []ConfigurationContext{
			{
				Name: "test",
				Services: []Service{
					{
						Name:                  "svc",
						HelmRepoPath:          "/tmp/helm",
						HelmBranch:            "main",
						HelmChartRelativePath: "charts",
						Certificates:          []CertificateRequest{invalidCert},
					},
				},
			},
		},
	}
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty k8sSecret.name")
}
