package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestConfig_ContextExists(t *testing.T) {
	contextName := string(uuid.NewUUID())
	config := Config{
		Contexts: []ConfigurationContext{
			{
				Name: contextName,
			},
		},
	}
	assert.True(t, config.ContextExists(contextName))
	assert.False(t, config.ContextExists(string(uuid.NewUUID())))
}

func TestConfig_GetContext(t *testing.T) {
	context := ConfigurationContext{
		Name: string(uuid.NewUUID()),
	}
	config := Config{
		Contexts: []ConfigurationContext{context},
	}
	actual, err := config.GetContext(context.Name)
	assert.Nil(t, err)
	assert.Equal(t, context, *actual)
	actual, err = config.GetContext(string(uuid.NewUUID()))
	assert.NotNil(t, err)
	assert.Nil(t, actual)
}

func TestConfigurationContext_GetService(t *testing.T) {
	context := ConfigurationContext{
		Services: []Service{
			{
				Name: string(uuid.NewUUID()),
			},
		},
	}
	actual := context.GetService(context.Services[0].Name)
	assert.Equal(t, context.Services[0], *actual)
	actual = context.GetService(string(uuid.NewUUID()))
	assert.Nil(t, actual)
}

func TestConfigurationContext_FilterServices_ByProfile(t *testing.T) {
	ctx := ConfigurationContext{
		Services: []Service{
			{Name: "svc-a", Profiles: []string{"infra", "all"}},
			{Name: "svc-b", Profiles: []string{"app", "all"}},
			{Name: "svc-c", Profiles: []string{"infra", "all"}},
		},
	}

	result := ctx.FilterServices(nil, "infra")

	assert.Len(t, result, 2)
	assert.Equal(t, "svc-a", result[0].Name)
	assert.Equal(t, "svc-c", result[1].Name)
}

func TestConfigurationContext_FilterServices_ByNames(t *testing.T) {
	ctx := ConfigurationContext{
		Services: []Service{
			{Name: "svc-a", Profiles: []string{"infra"}},
			{Name: "svc-b", Profiles: []string{"app"}},
			{Name: "svc-c", Profiles: []string{"infra"}},
		},
	}

	result := ctx.FilterServices([]string{"svc-b", "svc-c"}, "infra")

	assert.Len(t, result, 2)
	assert.Equal(t, "svc-b", result[0].Name)
	assert.Equal(t, "svc-c", result[1].Name)
}

func TestConfigurationContext_FilterServices_NamesNotFound(t *testing.T) {
	ctx := ConfigurationContext{
		Services: []Service{
			{Name: "svc-a", Profiles: []string{"all"}},
		},
	}

	result := ctx.FilterServices([]string{"nonexistent"}, "all")

	assert.Empty(t, result)
}

func TestConfigurationContext_FilterServices_EmptyServices(t *testing.T) {
	ctx := ConfigurationContext{}

	result := ctx.FilterServices(nil, "all")

	assert.Empty(t, result)
}

func TestConfigurationContext_FilterServices_AllProfile(t *testing.T) {
	ctx := ConfigurationContext{
		Services: []Service{
			{Name: "svc-a", Profiles: []string{"infra", "all"}},
			{Name: "svc-b", Profiles: []string{"app", "all"}},
		},
	}

	result := ctx.FilterServices(nil, "all")

	assert.Len(t, result, 2)
}

func TestCreateDefaultConfigReturnsConfig(t *testing.T) {
	defaultConfig := CreateDefaultConfig()
	assert.NotNil(t, defaultConfig)
	assert.Equal(t, 1, len(defaultConfig.Contexts))
	assert.Nil(t, defaultConfig.Validate())
}

func TestConfig_Validate_ContextNamePathTraversal(t *testing.T) {
	tests := []struct {
		name        string
		contextName string
		wantErr     bool
	}{
		{"valid name", "my-context", false},
		{"valid name with dashes", "my-context-123", false},
		{"path traversal with ..", "../etc", true},
		{"path traversal with forward slash", "foo/bar", true},
		{"path traversal with backslash", "foo\\bar", true},
		{"null byte injection", "foo\x00bar", true},
		{"double dot in middle", "foo..bar", true},
		{"empty name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Contexts: []ConfigurationContext{
					{
						Name: tt.contextName,
						Services: []Service{
							{
								Name:                  "test-svc",
								HelmRepoPath:          "/tmp/helm",
								HelmBranch:            "main",
								HelmChartRelativePath: "charts",
								DockerImages: []DockerImage{
									{
										Name:                     "test-img",
										DockerfilePath:           "Dockerfile",
										BuildContextRelativePath: ".",
										GitRepoPath:              "/tmp/repo",
										GitRef:                   "main",
									},
								},
							},
						},
					},
				},
			}

			err := config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_DockerImageNameRules(t *testing.T) {
	tests := []struct {
		name          string
		imageName     string
		wantErr       bool
		wantErrSubstr string
	}{
		{"plain name", "api", false, ""},
		{"namespaced", "henriq/foo-service", false, ""},
		{"registry prefixed", "registry.example.com/team/api", false, ""},
		{"with dots", "foo.bar.baz", false, ""},
		{"colon (tag-like)", "api:latest", true, "':' is not permitted"},
		{"trailing colon", "api:", true, "':' is not permitted"},
		{"leading colon", ":api", true, "':' is not permitted"},
		{"null byte", "api\x00x", true, "control character"},
		{"newline", "api\nx", true, "control character"},
		{"tab", "api\tx", true, "control character"},
		{"DEL", "api\x7fx", true, "control character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Contexts: []ConfigurationContext{
					{
						Name: "ctx",
						Services: []Service{
							{
								Name:                  "test-svc",
								HelmRepoPath:          "/tmp/helm",
								HelmBranch:            "main",
								HelmChartRelativePath: "charts",
								DockerImages: []DockerImage{
									{
										Name:                     tt.imageName,
										DockerfilePath:           "Dockerfile",
										BuildContextRelativePath: ".",
										GitRepoPath:              "/tmp/repo",
										GitRef:                   "main",
									},
								},
							},
						},
					},
				},
			}

			err := config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_ServiceNameRules(t *testing.T) {
	tests := []struct {
		name          string
		serviceName   string
		wantErr       bool
		wantErrSubstr string
	}{
		{"plain name", "api", false, ""},
		{"with dash", "my-api", false, ""},
		{"with underscore", "my_api", false, ""},
		{"with slash", "team/api", false, ""},
		{"colon", "api:v2", true, "':' is not permitted"},
		{"null byte", "api\x00x", true, "control character"},
		{"newline", "api\nx", true, "control character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Contexts: []ConfigurationContext{
					{
						Name: "ctx",
						Services: []Service{
							{
								Name:                  tt.serviceName,
								HelmRepoPath:          "/tmp/helm",
								HelmBranch:            "main",
								HelmChartRelativePath: "charts",
								DockerImages: []DockerImage{
									{
										Name:                     "img",
										DockerfilePath:           "Dockerfile",
										BuildContextRelativePath: ".",
										GitRepoPath:              "/tmp/repo",
										GitRef:                   "main",
									},
								},
							},
						},
					},
				},
			}

			err := config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_IsDeployable(t *testing.T) {
	tests := []struct {
		name    string
		service Service
		want    bool
	}{
		{"all helm fields empty is non-deployable", Service{Name: "svc"}, false},
		{"any helm field set marks service deployable (partials rejected by Validate)", Service{HelmRepoPath: "x"}, true},
		{"helmBranch set marks service deployable", Service{HelmBranch: "x"}, true},
		{"helmChartRelativePath set marks service deployable", Service{HelmChartRelativePath: "x"}, true},
		{
			"all helm fields set is deployable",
			Service{HelmRepoPath: "a", HelmBranch: "b", HelmChartRelativePath: "c"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.service.IsDeployable())
		})
	}
}

func TestConfig_Validate_NonDeployableService(t *testing.T) {
	baseImage := DockerImage{
		Name:                     "img",
		DockerfilePath:           "Dockerfile",
		BuildContextRelativePath: ".",
		GitRepoPath:              "/tmp/repo",
		GitRef:                   "main",
	}
	localPort := 8080

	tests := []struct {
		name    string
		service Service
		wantErr bool
	}{
		{
			name: "non-deployable with docker images is valid",
			service: Service{
				Name:         "automation",
				DockerImages: []DockerImage{baseImage},
			},
			wantErr: false,
		},
		{
			name:    "non-deployable with no docker images is valid",
			service: Service{Name: "automation"},
			wantErr: false,
		},
		{
			name: "partial helm config (only repo) is rejected",
			service: Service{
				Name:         "svc",
				HelmRepoPath: "/tmp/helm",
				DockerImages: []DockerImage{baseImage},
			},
			wantErr: true,
		},
		{
			name: "partial helm config (two of three) is rejected",
			service: Service{
				Name:                  "svc",
				HelmRepoPath:          "/tmp/helm",
				HelmChartRelativePath: "charts",
				DockerImages:          []DockerImage{baseImage},
			},
			wantErr: true,
		},
		{
			name: "non-deployable with localPort is rejected",
			service: Service{
				Name:      "automation",
				LocalPort: &localPort,
			},
			wantErr: true,
		},
		{
			name: "non-deployable with certificates is rejected",
			service: Service{
				Name: "automation",
				Certificates: []CertificateRequest{{
					Type:     CertificateTypeServer,
					DNSNames: []string{"example.svc"},
					K8sSecret: K8sSecretConfig{
						Name: "cert-secret",
						Type: K8sSecretTypeTLS,
					},
				}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Contexts: []ConfigurationContext{
					{Name: "ctx", Services: []Service{tt.service}},
				},
			}
			err := config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizePilotVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},
		{"  1.2.3  ", "v1.2.3"},
		{"", ""},
		{"v0.0.0-prerelease+abc", "v0.0.0-prerelease+abc"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, NormalizePilotVersion(tt.input), tt.input)
	}
}

func TestIsValidPilotVersion(t *testing.T) {
	assert.True(t, IsValidPilotVersion("v1.2.3"))
	assert.True(t, IsValidPilotVersion("1.2.3"))
	assert.True(t, IsValidPilotVersion("v1.2.3-prerelease+abc"))
	assert.False(t, IsValidPilotVersion("dev"))
	assert.False(t, IsValidPilotVersion("not-a-version"))
	assert.False(t, IsValidPilotVersion(""))
}

func TestHigherPilotVersion(t *testing.T) {
	tests := []struct {
		a, b, expected string
	}{
		{"", "", ""},
		{"v1.2.3", "", "v1.2.3"},
		{"", "v1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.4", "v1.2.4"},
		{"v2.0.0", "v1.9.9", "v2.0.0"},
		{"1.2.3", "v1.2.4", "v1.2.4"},
		{"v1.2.3", "v1.2.3", "v1.2.3"},
		{"v1.2.3-prerelease", "v1.2.3", "v1.2.3"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, HigherPilotVersion(tt.a, tt.b), "a=%s b=%s", tt.a, tt.b)
	}
}

func TestPilotVersionMeetsMinimum(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		minimum  string
		expected bool
	}{
		{"equal releases", "v1.8.12", "v1.8.12", true},
		{"current newer", "v1.9.0", "v1.8.12", true},
		{"current older", "v1.8.11", "v1.8.12", false},
		{"prerelease of same release meets minimum", "v1.8.12-prerelease+e706e158", "v1.8.12", true},
		{"prerelease of newer release meets minimum", "v1.9.0-prerelease+abc", "v1.8.12", true},
		{"prerelease of older release fails minimum", "v1.8.11-prerelease+abc", "v1.8.12", false},
		{"release meets prerelease minimum of same X.Y.Z", "v1.8.12", "v1.8.12-rc.1", true},
		{"prerelease meets prerelease minimum of same X.Y.Z", "v1.8.12-prerelease+abc", "v1.8.12-rc.1", true},
		{"different prerelease tags of same X.Y.Z are equivalent", "v1.8.12-alpha", "v1.8.12-rc.1", true},
		{"build metadata only equals release", "v1.8.12+abc", "v1.8.12", true},
		{"empty current fails any valid minimum", "", "v1.8.12", false},
		{"both empty", "", "", true},
		{"current without v prefix", "1.8.12", "v1.8.12", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PilotVersionMeetsMinimum(tt.current, tt.minimum))
		})
	}
}

func TestConfig_EffectiveMinPilotVersion(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name:     "all empty",
			config:   Config{Contexts: []ConfigurationContext{{Name: "a"}, {Name: "b"}}},
			expected: "",
		},
		{
			name: "root only",
			config: Config{
				MinPilotVersion: "v1.2.3",
				Contexts:        []ConfigurationContext{{Name: "a"}},
			},
			expected: "v1.2.3",
		},
		{
			name: "context only",
			config: Config{
				Contexts: []ConfigurationContext{{Name: "a", MinPilotVersion: "v1.5.0"}},
			},
			expected: "v1.5.0",
		},
		{
			name: "context wins over root",
			config: Config{
				MinPilotVersion: "v1.0.0",
				Contexts: []ConfigurationContext{
					{Name: "a", MinPilotVersion: "v0.9.0"},
					{Name: "b", MinPilotVersion: "v2.0.0"},
				},
			},
			expected: "v2.0.0",
		},
		{
			name: "root wins over contexts",
			config: Config{
				MinPilotVersion: "v3.0.0",
				Contexts: []ConfigurationContext{
					{Name: "a", MinPilotVersion: "v1.5.0"},
					{Name: "b", MinPilotVersion: "v2.0.0"},
				},
			},
			expected: "v3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.EffectiveMinPilotVersion())
		})
	}
}

func TestConfig_Validate_MinPilotVersion(t *testing.T) {
	baseService := Service{
		Name:                  "test-svc",
		HelmRepoPath:          "/tmp/helm",
		HelmBranch:            "main",
		HelmChartRelativePath: "charts",
		DockerImages: []DockerImage{
			{
				Name:                     "test-img",
				DockerfilePath:           "Dockerfile",
				BuildContextRelativePath: ".",
				GitRepoPath:              "/tmp/repo",
				GitRef:                   "main",
			},
		},
	}

	tests := []struct {
		name           string
		rootMinVersion string
		ctxMinVersion  string
		wantErr        bool
	}{
		{name: "both empty", wantErr: false},
		{name: "root valid", rootMinVersion: "v1.2.3", wantErr: false},
		{name: "root valid no prefix", rootMinVersion: "1.2.3", wantErr: false},
		{name: "ctx valid", ctxMinVersion: "v1.2.3", wantErr: false},
		{name: "root invalid", rootMinVersion: "not-a-version", wantErr: true},
		{name: "ctx invalid", ctxMinVersion: "1..2.3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				MinPilotVersion: tt.rootMinVersion,
				Contexts: []ConfigurationContext{
					{
						Name:            "ctx",
						MinPilotVersion: tt.ctxMinVersion,
						Services:        []Service{baseService},
					},
				},
			}
			err := config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_DockerImageInheritsGitFieldsFromService(t *testing.T) {
	tests := []struct {
		name               string
		serviceGitRepoPath string
		serviceGitRef      string
		imageGitRepoPath   string
		imageGitRef        string
		wantErrSubstr      string
	}{
		{"image supplies both", "", "", "/tmp/repo", "main", ""},
		{"service supplies both", "/tmp/repo", "main", "", "", ""},
		{"image overrides service", "/tmp/svc-repo", "svc-main", "/tmp/repo", "main", ""},
		{"neither supplies repo path", "", "main", "", "main", "empty gitRepoPath"},
		{"neither supplies ref", "/tmp/repo", "", "/tmp/repo", "", "empty gitRef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Contexts: []ConfigurationContext{
					{
						Name: "ctx",
						Services: []Service{
							{
								Name:                  "test-svc",
								HelmRepoPath:          "/tmp/helm",
								HelmBranch:            "main",
								HelmChartRelativePath: "charts",
								GitRepoPath:           tt.serviceGitRepoPath,
								GitRef:                tt.serviceGitRef,
								DockerImages: []DockerImage{
									{
										Name:                     "api",
										DockerfilePath:           "Dockerfile",
										BuildContextRelativePath: ".",
										GitRepoPath:              tt.imageGitRepoPath,
										GitRef:                   tt.imageGitRef,
									},
								},
							},
						},
					},
				},
			}

			err := config.Validate()

			if tt.wantErrSubstr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrSubstr)
		})
	}
}

func TestConfig_Validate_RejectsServiceNameWithParentDirectory(t *testing.T) {
	config := Config{
		Contexts: []ConfigurationContext{
			{
				Name: "ctx",
				Services: []Service{
					{
						Name:                  "../../escape",
						HelmRepoPath:          "/tmp/helm",
						HelmBranch:            "main",
						HelmChartRelativePath: "charts",
					},
				},
			},
		},
	}

	err := config.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "'..' is not permitted")
}
