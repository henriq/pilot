package core

import (
	"encoding/base64"
	"strings"
	"testing"

	"dx/internal/core/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderCertificateSecretManifests_TLSSecret(t *testing.T) {
	certs := []domain.ProvisionedCertificate{
		{
			Request: domain.CertificateRequest{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"foo.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "foo-tls",
					Type: domain.K8sSecretTypeTLS,
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte("cert-pem"),
				"tls.key": []byte("key-pem"),
				"ca.crt":  []byte("ca-pem"),
			},
		},
	}

	result, err := RenderCertificateSecretManifests(certs)

	require.NoError(t, err)
	yaml := string(result)
	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: Secret")
	assert.Contains(t, yaml, "name: foo-tls")
	assert.Contains(t, yaml, "managed-by: dx")
	assert.Contains(t, yaml, "type: kubernetes.io/tls")
	assert.Contains(t, yaml, "ca.crt: "+base64.StdEncoding.EncodeToString([]byte("ca-pem")))
	assert.Contains(t, yaml, "tls.crt: "+base64.StdEncoding.EncodeToString([]byte("cert-pem")))
	assert.Contains(t, yaml, "tls.key: "+base64.StdEncoding.EncodeToString([]byte("key-pem")))
}

func TestRenderCertificateSecretManifests_OpaqueSecret(t *testing.T) {
	certs := []domain.ProvisionedCertificate{
		{
			Request: domain.CertificateRequest{
				Type:     domain.CertificateTypeClient,
				DNSNames: []string{"bar.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "bar-certs",
					Type: domain.K8sSecretTypeOpaque,
					Keys: &domain.OpaqueSecretKeys{
						PrivateKey: "client.key",
						Cert:       "client.crt",
						CA:         "client-ca.crt",
					},
				},
			},
			Data: map[string][]byte{
				"client.crt":    []byte("cert"),
				"client.key":    []byte("key"),
				"client-ca.crt": []byte("ca"),
			},
		},
	}

	result, err := RenderCertificateSecretManifests(certs)

	require.NoError(t, err)
	yaml := string(result)
	assert.Contains(t, yaml, "name: bar-certs")
	assert.Contains(t, yaml, "type: Opaque")
	assert.Contains(t, yaml, "client.crt: "+base64.StdEncoding.EncodeToString([]byte("cert")))
	assert.Contains(t, yaml, "client.key: "+base64.StdEncoding.EncodeToString([]byte("key")))
	assert.Contains(t, yaml, "client-ca.crt: "+base64.StdEncoding.EncodeToString([]byte("ca")))
}

func TestRenderCertificateSecretManifests_MultipleCerts(t *testing.T) {
	certs := []domain.ProvisionedCertificate{
		{
			Request: domain.CertificateRequest{
				K8sSecret: domain.K8sSecretConfig{Name: "first-tls", Type: domain.K8sSecretTypeTLS},
			},
			Data: map[string][]byte{"tls.crt": []byte("c1"), "tls.key": []byte("k1"), "ca.crt": []byte("ca1")},
		},
		{
			Request: domain.CertificateRequest{
				K8sSecret: domain.K8sSecretConfig{Name: "second-tls", Type: domain.K8sSecretTypeTLS},
			},
			Data: map[string][]byte{"tls.crt": []byte("c2"), "tls.key": []byte("k2"), "ca.crt": []byte("ca2")},
		},
	}

	result, err := RenderCertificateSecretManifests(certs)

	require.NoError(t, err)
	yaml := string(result)
	assert.Contains(t, yaml, "name: first-tls")
	assert.Contains(t, yaml, "name: second-tls")
	assert.Equal(t, 1, strings.Count(yaml, "---"), "multiple secrets should be separated by ---")
}

func TestRenderCertificateSecretManifests_EmptySlice(t *testing.T) {
	result, err := RenderCertificateSecretManifests(nil)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRenderCertificateSecretManifests_UnsupportedSecretType(t *testing.T) {
	certs := []domain.ProvisionedCertificate{
		{
			Request: domain.CertificateRequest{
				K8sSecret: domain.K8sSecretConfig{Name: "bad", Type: "invalid"},
			},
			Data: map[string][]byte{"key": []byte("val")},
		},
	}

	_, err := RenderCertificateSecretManifests(certs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported k8sSecret.type")
}
