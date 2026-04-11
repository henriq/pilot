package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sort"

	"pilot/internal/core/domain"
)

// RenderCertificateSecretManifests renders provisioned certificates as K8s Secret YAML manifests.
// The output can be included as a template in a Helm wrapper chart.
func RenderCertificateSecretManifests(certs []domain.ProvisionedCertificate) ([]byte, error) {
	if len(certs) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer
	for i, cert := range certs {
		if i > 0 {
			buf.WriteString("---\n")
		}

		secretType, err := k8sSecretTypeString(cert.Request.K8sSecret.Type)
		if err != nil {
			return nil, err
		}

		buf.WriteString("apiVersion: v1\n")
		buf.WriteString("kind: Secret\n")
		buf.WriteString("metadata:\n")
		fmt.Fprintf(&buf, "  name: %s\n", cert.Request.K8sSecret.Name)
		buf.WriteString("  labels:\n")
		buf.WriteString("    managed-by: pilot\n")
		fmt.Fprintf(&buf, "type: %s\n", secretType)
		buf.WriteString("data:\n")

		// Sort keys for deterministic output
		keys := make([]string, 0, len(cert.Data))
		for k := range cert.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			encoded := base64.StdEncoding.EncodeToString(cert.Data[key])
			fmt.Fprintf(&buf, "  %s: %s\n", key, encoded)
		}
	}

	return buf.Bytes(), nil
}

func k8sSecretTypeString(t domain.K8sSecretType) (string, error) {
	switch t {
	case domain.K8sSecretTypeTLS:
		return "kubernetes.io/tls", nil
	case domain.K8sSecretTypeOpaque:
		return "Opaque", nil
	default:
		return "", fmt.Errorf("unsupported k8sSecret.type '%s'", t)
	}
}
