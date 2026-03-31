package core

import (
	"fmt"
	"strings"
	"testing"

	"dx/internal/core/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevProxyConfigGenerator_Generate_Success(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-1",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-1"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, true)

	require.NoError(t, err)
	assert.NotNil(t, configs)
	assert.NotEmpty(t, configs.HAProxyConfig)
	assert.NotEmpty(t, configs.HAProxyDockerfile)
	assert.NotEmpty(t, configs.MitmProxyDockerfile)
	assert.NotEmpty(t, configs.HelmChartYaml)
	assert.NotEmpty(t, configs.HelmDeploymentYaml)
}

func TestDevProxyConfigGenerator_Generate_NoInterception(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-1",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-1"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, false)

	require.NoError(t, err)
	assert.NotNil(t, configs)
	assert.NotEmpty(t, configs.HAProxyConfig)
	assert.NotEmpty(t, configs.HAProxyDockerfile)
	assert.Nil(t, configs.MitmProxyDockerfile, "MitmProxyDockerfile should be nil when not intercepting")
	assert.NotEmpty(t, configs.HelmChartYaml)
	assert.NotEmpty(t, configs.HelmDeploymentYaml)
}

func TestDevProxyConfigGenerator_Generate_HAProxyConfigContent(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "my-service",
				KubernetesPort:  9090,
				LocalPort:       4000,
				HealthCheckPath: "/healthz",
				Selector:        map[string]string{"app": "my-service"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, false)

	require.NoError(t, err)
	haproxyConfig := string(configs.HAProxyConfig)

	// Verify HAProxy config contains expected service configuration
	assert.Contains(t, haproxyConfig, "my-service", "HAProxy config should contain service name")
	assert.Contains(t, haproxyConfig, fmt.Sprintf("%d", DevProxyHAProxyStartPort), "HAProxy config should contain HAProxy start port")
	// Verify mitmweb proxy is always present regardless of interception flag
	assert.Contains(t, haproxyConfig, "mitmweb-proxy", "HAProxy config should contain mitmweb proxy frontend")
	assert.Contains(t, haproxyConfig, "HTTP Interception Not Enabled", "HAProxy config should contain mitmweb fallback response")
}

func TestDevProxyConfigGenerator_Generate_PortIncrement(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-1",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-1"},
			},
			{
				Name:            "service-2",
				KubernetesPort:  8081,
				LocalPort:       3001,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-2"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, false)

	require.NoError(t, err)
	haproxyConfig := string(configs.HAProxyConfig)

	// Verify both services are present with incremented ports
	assert.Contains(t, haproxyConfig, "service-1")
	assert.Contains(t, haproxyConfig, "service-2")
	// First service gets port 8080, second gets 8081
	assert.Contains(t, haproxyConfig, "8080")
	assert.Contains(t, haproxyConfig, "8081")
}

func TestDevProxyConfigGenerator_Generate_ChecksumDeterministic(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-1",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-1"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs1, err := sut.Generate(configContext, false)
	require.NoError(t, err)

	configs2, err := sut.Generate(configContext, false)
	require.NoError(t, err)

	// Same input should produce same output (deterministic checksum)
	assert.Equal(t, configs1.HelmDeploymentYaml, configs2.HelmDeploymentYaml)
}

func TestDevProxyConfigGenerator_Generate_HelmChartYamlContent(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "my-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-1",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-1"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, false)

	require.NoError(t, err)
	chartYaml := string(configs.HelmChartYaml)

	// Verify Chart.yaml contains expected fields
	assert.Contains(t, chartYaml, "name:")
	assert.Contains(t, chartYaml, "version:")
}

func TestDevProxyConfigGenerator_Generate_EmptyLocalServices(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name:          "test-context",
		LocalServices: []domain.LocalService{},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, false)

	require.NoError(t, err)
	assert.NotNil(t, configs)
	// Should still generate valid configs even with no services
	assert.NotEmpty(t, configs.HAProxyConfig)
	assert.NotEmpty(t, configs.HelmChartYaml)
}

func TestDevProxyConfigGenerator_Generate_SpecialCharactersInServiceName(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-with-dashes",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-with-dashes"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configs, err := sut.Generate(configContext, false)

	require.NoError(t, err)
	haproxyConfig := string(configs.HAProxyConfig)
	assert.Contains(t, haproxyConfig, "service-with-dashes")
}

func TestDevProxyConfigGenerator_buildTemplateValues_PortAssignment(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{Name: "svc1", KubernetesPort: 80, LocalPort: 3000, HealthCheckPath: "/", Selector: map[string]string{"app": "svc1"}},
			{Name: "svc2", KubernetesPort: 81, LocalPort: 3001, HealthCheckPath: "/", Selector: map[string]string{"app": "svc2"}},
			{Name: "svc3", KubernetesPort: 82, LocalPort: 3002, HealthCheckPath: "/", Selector: map[string]string{"app": "svc3"}},
		},
	}

	sut := ProvideDevProxyConfigGenerator()
	values := sut.buildTemplateValues(configContext, false)

	services := values["Services"].([]map[string]interface{})
	assert.Len(t, services, 3)

	// Verify port increments
	assert.Equal(t, DevProxyHAProxyStartPort, services[0]["FrontendPort"])
	assert.Equal(t, DevProxyMitmproxyStartPort, services[0]["ProxyPort"])

	assert.Equal(t, DevProxyHAProxyStartPort+1, services[1]["FrontendPort"])
	assert.Equal(t, DevProxyMitmproxyStartPort+1, services[1]["ProxyPort"])

	assert.Equal(t, DevProxyHAProxyStartPort+2, services[2]["FrontendPort"])
	assert.Equal(t, DevProxyMitmproxyStartPort+2, services[2]["ProxyPort"])
}

func TestDevProxyConfigGenerator_buildTemplateValues_Checksum(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{Name: "svc1", KubernetesPort: 80, LocalPort: 3000, HealthCheckPath: "/", Selector: map[string]string{"app": "svc1"}},
		},
	}

	sut := ProvideDevProxyConfigGenerator()
	values := sut.buildTemplateValues(configContext, false)

	checksum := values["Checksum"].(string)
	assert.Len(t, checksum, 62, "Checksum should be 62 characters (truncated SHA256 hex)")
	assert.True(t, isHexString(checksum), "Checksum should be a valid hex string")
}

func TestDevProxyConfigGenerator_GenerateChecksum_DiffersWithInterceptHttp(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{Name: "svc1", KubernetesPort: 80, LocalPort: 3000, HealthCheckPath: "/", Selector: map[string]string{"app": "svc1"}},
		},
	}

	sut := ProvideDevProxyConfigGenerator()
	checksumWithout := sut.GenerateChecksum(configContext, false)
	checksumWith := sut.GenerateChecksum(configContext, true)

	assert.NotEqual(t, checksumWithout, checksumWith, "Checksums should differ when interceptHttp changes")
}

func TestDevProxyConfigGenerator_Generate_HelmDeploymentYamlDiffersWithInterceptHttp(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:            "service-1",
				KubernetesPort:  8080,
				LocalPort:       3000,
				HealthCheckPath: "/health",
				Selector:        map[string]string{"app": "service-1"},
			},
		},
	}

	sut := ProvideDevProxyConfigGenerator()

	configsWithout, err := sut.Generate(configContext, false)
	require.NoError(t, err)

	configsWith, err := sut.Generate(configContext, true)
	require.NoError(t, err)

	deploymentWithout := string(configsWithout.HelmDeploymentYaml)
	deploymentWith := string(configsWith.HelmDeploymentYaml)

	assert.NotContains(t, deploymentWithout, "mitmproxy", "Deployment YAML should not contain mitmproxy when interception is disabled")
	assert.Contains(t, deploymentWith, "mitmproxy", "Deployment YAML should contain mitmproxy when interception is enabled")
}

func TestDevProxyConfigGenerator_buildTemplateValues_InterceptHttpFlag(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name:          "test-context",
		LocalServices: []domain.LocalService{},
	}

	sut := ProvideDevProxyConfigGenerator()

	valuesWithout := sut.buildTemplateValues(configContext, false)
	valuesWith := sut.buildTemplateValues(configContext, true)

	assert.Equal(t, false, valuesWithout["InterceptHttp"])
	assert.Equal(t, true, valuesWith["InterceptHttp"])
}

func isHexString(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func TestTemplateFunctions_Indent_NormalizesLineEndings(t *testing.T) {
	indentFunc := templateFunctions["indent"].(func(int, string) string)

	tests := []struct {
		name     string
		indent   int
		input    string
		expected string
	}{
		{
			name:     "unix line endings",
			indent:   2,
			input:    "line1\nline2\nline3",
			expected: "  line1\n  line2\n  line3",
		},
		{
			name:     "windows CRLF line endings",
			indent:   2,
			input:    "line1\r\nline2\r\nline3",
			expected: "  line1\n  line2\n  line3",
		},
		{
			name:     "old mac CR line endings",
			indent:   2,
			input:    "line1\rline2\rline3",
			expected: "  line1\n  line2\n  line3",
		},
		{
			name:     "mixed line endings",
			indent:   2,
			input:    "line1\r\nline2\nline3\rline4",
			expected: "  line1\n  line2\n  line3\n  line4",
		},
		{
			name:     "single line no ending",
			indent:   4,
			input:    "single line",
			expected: "    single line",
		},
		{
			name:     "empty string",
			indent:   2,
			input:    "",
			expected: "  ",
		},
		{
			name:     "zero indent",
			indent:   0,
			input:    "line1\nline2",
			expected: "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indentFunc(tt.indent, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
