package core

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"dx/internal/core/domain"

	"gopkg.in/yaml.v3"
)

const (
	// DevProxyHAProxyStartPort is the starting port for HAProxy frontend listeners.
	DevProxyHAProxyStartPort = 8080
	// DevProxyMitmproxyStartPort is the starting port for mitmproxy backends.
	DevProxyMitmproxyStartPort = 18080
)

//go:embed templates/dev-proxy/*/*.tpl
var templateFiles embed.FS

// DevProxyConfigs holds all generated configuration files for the dev-proxy.
type DevProxyConfigs struct {
	HAProxyConfig       []byte
	HAProxyDockerfile   []byte
	MitmProxyDockerfile []byte
	HelmChartYaml       []byte
	HelmDeploymentYaml  []byte
	Password            string
}

// DevProxyConfigGenerator generates dev-proxy configuration files from domain configuration.
type DevProxyConfigGenerator struct{}

// ProvideDevProxyConfigGenerator creates a new DevProxyConfigGenerator.
func ProvideDevProxyConfigGenerator() *DevProxyConfigGenerator {
	return &DevProxyConfigGenerator{}
}

// Generate creates all dev-proxy configuration files from the given configuration context.
// When interceptHttp is true, mitmproxy configuration is also generated using the provided password.
// Returns a DevProxyConfigs struct containing all generated content.
func (g *DevProxyConfigGenerator) Generate(configContext *domain.ConfigurationContext, interceptHttp bool, password string, certificateSecrets []byte) (*DevProxyConfigs, error) {
	values := g.buildTemplateValues(configContext, interceptHttp, password, certificateSecrets)

	haproxyConfig, err := renderTemplate("templates/dev-proxy/haproxy/haproxy.cfg.tpl", values)
	if err != nil {
		return nil, fmt.Errorf("failed to render haproxy config: %w", err)
	}

	haproxyDockerfile, err := renderTemplate("templates/dev-proxy/haproxy/Dockerfile.tpl", values)
	if err != nil {
		return nil, fmt.Errorf("failed to render haproxy dockerfile: %w", err)
	}

	var mitmproxyDockerfile []byte
	if interceptHttp {
		mitmproxyDockerfile, err = renderTemplate("templates/dev-proxy/mitmproxy/Dockerfile.tpl", values)
		if err != nil {
			return nil, fmt.Errorf("failed to render mitmproxy dockerfile: %w", err)
		}
	}

	helmChartYaml, err := renderTemplate("templates/dev-proxy/helm/Chart.yaml.tpl", values)
	if err != nil {
		return nil, fmt.Errorf("failed to render helm chart.yaml: %w", err)
	}

	helmDeploymentYaml, err := renderTemplate("templates/dev-proxy/helm/deployment.yaml.tpl", values)
	if err != nil {
		return nil, fmt.Errorf("failed to render helm deployment.yaml: %w", err)
	}

	return &DevProxyConfigs{
		HAProxyConfig:       haproxyConfig,
		HAProxyDockerfile:   haproxyDockerfile,
		MitmProxyDockerfile: mitmproxyDockerfile,
		HelmChartYaml:       helmChartYaml,
		HelmDeploymentYaml:  helmDeploymentYaml,
		Password:            password,
	}, nil
}

// GenerateChecksum computes the configuration checksum for a given context.
// This checksum is used to detect configuration changes for the dev-proxy deployment.
// The checksum is a SHA256 hash of the LocalServices configuration and interceptHttp flag,
// truncated to 62 characters for readability and to ensure it fits within common annotation
// display widths.
func (g *DevProxyConfigGenerator) GenerateChecksum(configContext *domain.ConfigurationContext, interceptHttp bool, certificateSecrets []byte) string {
	hash := sha256.New()
	// Error can be safely ignored: LocalServices contains only JSON-serializable primitive types
	// (strings, ints, and map[string]string). json.Marshal cannot fail for these types.
	serviceJSON, _ := json.Marshal(configContext.LocalServices)
	hash.Write(serviceJSON)
	if interceptHttp {
		hash.Write([]byte{1})
	} else {
		hash.Write([]byte{0})
	}
	if len(certificateSecrets) > 0 {
		hash.Write(certificateSecrets)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))[:62]
}

// buildTemplateValues constructs the values map for template rendering.
func (g *DevProxyConfigGenerator) buildTemplateValues(configContext *domain.ConfigurationContext, interceptHttp bool, password string, certificateSecrets []byte) map[string]interface{} {
	frontendPort := DevProxyHAProxyStartPort
	proxyPort := DevProxyMitmproxyStartPort
	services := make([]map[string]interface{}, len(configContext.LocalServices))

	for i, localService := range configContext.LocalServices {
		services[i] = map[string]interface{}{
			"Name":            localService.Name,
			"FrontendPort":    frontendPort,
			"ProxyPort":       proxyPort,
			"KubernetesPort":  localService.KubernetesPort,
			"LocalPort":       localService.LocalPort,
			"HealthCheckPath": localService.HealthCheckPath,
			"Selector":        localService.Selector,
		}
		frontendPort++
		proxyPort++
	}

	checksum := g.GenerateChecksum(configContext, interceptHttp, certificateSecrets)

	return map[string]interface{}{
		"Services":      services,
		"Name":          configContext.Name,
		"Checksum":      checksum,
		"InterceptHttp": interceptHttp,
		"Password":      password,
		"TLSSecretName": InternalTLSSecretName,
	}
}

var templateFunctions = template.FuncMap{
	"toYaml": func(v interface{}) string {
		var buf strings.Builder
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(v); err != nil {
			// Return error marker that will be visible in output and cause YAML parsing to fail.
			// This is preferable to silent failure with empty string.
			return fmt.Sprintf("# ERROR: failed to encode YAML: %v", err)
		}
		return buf.String()
	},
	"indent": func(indent int, s string) string {
		// Normalize line endings to handle both Unix (\n) and Windows (\r\n)
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			lines[i] = strings.Repeat(" ", indent) + line
		}
		return strings.Join(lines, "\n")
	},
}

func renderTemplate(templatePath string, values map[string]interface{}) ([]byte, error) {
	templateFile, err := templateFiles.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	tmpl, err := template.New(templatePath).Funcs(templateFunctions).Parse(string(templateFile))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, values); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return []byte(result.String()), nil
}
