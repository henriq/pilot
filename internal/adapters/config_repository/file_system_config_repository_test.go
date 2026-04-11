package config_repository

import (
	"path/filepath"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/ports"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandImportPath(t *testing.T) {
	home := "/home/user"

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "tilde with path",
			path:     "~/projects/config.yaml",
			expected: "/home/user/projects/config.yaml",
		},
		{
			name:     "tilde only",
			path:     "~",
			expected: "/home/user",
		},
		{
			name:     "absolute path unchanged",
			path:     "/etc/config.yaml",
			expected: "/etc/config.yaml",
		},
		{
			name:     "relative path unchanged",
			path:     "relative/config.yaml",
			expected: "relative/config.yaml",
		},
		{
			name:     "tilde in middle unchanged",
			path:     "/some/~/path",
			expected: "/some/~/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandImportPath(tt.path, home)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandImportPath_WindowsStyle(t *testing.T) {
	// Test with Windows-style home directory
	home := `C:\Users\testuser`

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "tilde with backslash",
			path:     `~\projects\config.yaml`,
			expected: filepath.Join(home, `projects\config.yaml`),
		},
		{
			name:     "tilde with forward slash on windows home",
			path:     "~/projects/config.yaml",
			expected: filepath.Join(home, "projects/config.yaml"),
		},
		{
			name:     "tilde only",
			path:     "~",
			expected: home,
		},
		{
			name:     "windows absolute path unchanged",
			path:     `C:\Program Files\config.yaml`,
			expected: `C:\Program Files\config.yaml`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandImportPath(tt.path, home)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateContextName(t *testing.T) {
	tests := []struct {
		name      string
		context   string
		expectErr bool
	}{
		{name: "valid simple name", context: "production", expectErr: false},
		{name: "valid name with hyphen", context: "my-context", expectErr: false},
		{name: "valid name with underscore", context: "my_context", expectErr: false},
		{name: "empty name", context: "", expectErr: true},
		{name: "path traversal with dots", context: "../etc", expectErr: true},
		{name: "path with forward slash", context: "foo/bar", expectErr: true},
		{name: "path with backslash", context: "foo\\bar", expectErr: true},
		{name: "path with null byte", context: "foo\x00bar", expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateContextName(tt.context)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// mockTemplater implements ports.Templater for testing
type mockTemplater struct{}

func (m *mockTemplater) Render(template string, templateName string, values map[string]interface{}) (string, error) {
	return template, nil
}

// mockSecretsRepository implements ports.SecretsRepository for testing
type mockSecretsRepository struct {
	secrets []*domain.Secret
	err     error
}

func (m *mockSecretsRepository) LoadSecrets(configContextName string) ([]*domain.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.secrets, nil
}

func (m *mockSecretsRepository) SaveSecrets(secrets []*domain.Secret, configContextName string) error {
	return m.err
}

func TestFileSystemConfigRepository_SaveAndLoadCurrentContextName(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Save a valid context name
	err := repo.SaveCurrentContextName("test-context")
	require.NoError(t, err)

	// Load it back
	name, err := repo.LoadCurrentContextName()
	require.NoError(t, err)
	assert.Equal(t, "test-context", name)
}

func TestFileSystemConfigRepository_SaveCurrentContextName_RejectsInvalid(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	tests := []struct {
		name        string
		contextName string
	}{
		{"path traversal", "../etc"},
		{"forward slash", "foo/bar"},
		{"backslash", "foo\\bar"},
		{"null byte", "foo\x00bar"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.SaveCurrentContextName(tt.contextName)
			assert.Error(t, err, "should reject invalid context name: %s", tt.contextName)
		})
	}
}

func TestFileSystemConfigRepository_LoadCurrentContextName_RejectsInvalidStoredValue(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Write an invalid context name directly to the file (bypassing validation)
	currentContextPath := filepath.Join("~", ".pilot", "current-context")
	err := fs.WriteFile(currentContextPath, []byte("../malicious"), ports.ReadWrite)
	require.NoError(t, err)

	// LoadCurrentContextName should reject the invalid value
	_, err = repo.LoadCurrentContextName()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid context name")
}

func TestFileSystemConfigRepository_ConfigExists(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Config should not exist initially
	exists, err := repo.ConfigExists()
	require.NoError(t, err)
	assert.False(t, exists)

	// Write a config file
	configPath := filepath.Join("~", ".pilot-config.yaml")
	err = fs.WriteFile(configPath, []byte("contexts: []"), ports.ReadWrite)
	require.NoError(t, err)

	// Now it should exist
	exists, err = repo.ConfigExists()
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFileSystemConfigRepository_SaveConfig(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	config := domain.CreateDefaultConfig()
	err := repo.SaveConfig(&config)
	require.NoError(t, err)

	// Verify file was written
	exists, err := repo.ConfigExists()
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFileSystemConfigRepository_LoadEnvKey(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Try to load env key before it exists
	_, err := repo.LoadEnvKey("test-context")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "env-key does not exist")

	// Write an env key
	envKeyPath := filepath.Join("~", ".pilot", "test-context", "env-key")
	err = fs.WriteFile(envKeyPath, []byte("test-key-value"), ports.ReadWrite)
	require.NoError(t, err)

	// Load it back
	key, err := repo.LoadEnvKey("test-context")
	require.NoError(t, err)
	assert.Equal(t, "test-key-value", key)
}

func TestMergeConfigurationContexts(t *testing.T) {
	base := domain.ConfigurationContext{
		Name: "base",
		Scripts: map[string]string{
			"build": "echo base",
			"test":  "echo test",
		},
		Services: []domain.Service{
			{Name: "svc1", GitRef: "main", HelmBranch: "main"},
		},
		LocalServices: []domain.LocalService{
			{Name: "local1"},
		},
	}

	overlay := domain.ConfigurationContext{
		Name: "overlay",
		Scripts: map[string]string{
			"build":  "echo overlay", // Override
			"deploy": "echo deploy",  // New
		},
		Services: []domain.Service{
			{Name: "svc1", GitRef: "feature"}, // Override gitRef
		},
		LocalServices: []domain.LocalService{
			{Name: "local2"}, // Additional
		},
	}

	result := mergeConfigurationContexts(base, overlay)

	// Name should be overridden
	assert.Equal(t, "overlay", result.Name)

	// Scripts should be merged
	assert.Equal(t, "echo overlay", result.Scripts["build"])
	assert.Equal(t, "echo test", result.Scripts["test"])
	assert.Equal(t, "echo deploy", result.Scripts["deploy"])

	// Services should be overlaid
	require.Len(t, result.Services, 1)
	assert.Equal(t, "feature", result.Services[0].GitRef)
	assert.Equal(t, "main", result.Services[0].HelmBranch) // Not overridden

	// LocalServices should be appended
	assert.Len(t, result.LocalServices, 2)
}

func TestOverlayService(t *testing.T) {
	base := domain.Service{
		Name:        "base-svc",
		GitRepoPath: "git@example.com:base/repo.git",
		GitRef:      "main",
		HelmBranch:  "main",
	}

	overlay := domain.Service{
		Name:   "overlay-svc", // Override
		GitRef: "feature",     // Override
		// GitRepoPath not set - should keep base value
		// HelmBranch not set - should keep base value
	}

	result := overlayService(base, overlay)

	assert.Equal(t, "overlay-svc", result.Name)
	assert.Equal(t, "feature", result.GitRef)
	assert.Equal(t, "git@example.com:base/repo.git", result.GitRepoPath)
	assert.Equal(t, "main", result.HelmBranch)
}

func TestOverlayService_CertificatesMergedBySecretName(t *testing.T) {
	base := domain.Service{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"base.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "cert-a",
					Type: domain.K8sSecretTypeTLS,
				},
			},
			{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"other.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "cert-b",
					Type: domain.K8sSecretTypeTLS,
				},
			},
		},
	}

	overlay := domain.Service{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				// Override only DNSNames for cert-a
				DNSNames: []string{"overlay.localhost", "extra.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "cert-a",
				},
			},
		},
	}

	result := overlayService(base, overlay)

	require.Len(t, result.Certificates, 2)

	// cert-a: DNSNames overridden, Type kept from base
	assert.Equal(t, "cert-a", result.Certificates[0].K8sSecret.Name)
	assert.Equal(t, []string{"overlay.localhost", "extra.localhost"}, result.Certificates[0].DNSNames)
	assert.Equal(t, domain.CertificateTypeServer, result.Certificates[0].Type)
	assert.Equal(t, domain.K8sSecretTypeTLS, result.Certificates[0].K8sSecret.Type)

	// cert-b: unchanged
	assert.Equal(t, "cert-b", result.Certificates[1].K8sSecret.Name)
	assert.Equal(t, []string{"other.localhost"}, result.Certificates[1].DNSNames)
}

func TestOverlayService_CertificatesUnmatchedOverlayIgnored(t *testing.T) {
	base := domain.Service{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"base.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "cert-a",
					Type: domain.K8sSecretTypeTLS,
				},
			},
		},
	}

	overlay := domain.Service{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				DNSNames: []string{"new.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "cert-unknown",
				},
			},
		},
	}

	result := overlayService(base, overlay)

	// Base certificate unchanged, unmatched overlay ignored
	require.Len(t, result.Certificates, 1)
	assert.Equal(t, "cert-a", result.Certificates[0].K8sSecret.Name)
	assert.Equal(t, []string{"base.localhost"}, result.Certificates[0].DNSNames)
}

func TestOverlayService_CertificatesNilOverlayKeepsBase(t *testing.T) {
	base := domain.Service{
		Name: "svc",
		Certificates: []domain.CertificateRequest{
			{
				Type:     domain.CertificateTypeServer,
				DNSNames: []string{"base.localhost"},
				K8sSecret: domain.K8sSecretConfig{
					Name: "cert-a",
					Type: domain.K8sSecretTypeTLS,
				},
			},
		},
	}

	overlay := domain.Service{
		Name: "svc",
		// Certificates not set
	}

	result := overlayService(base, overlay)

	require.Len(t, result.Certificates, 1)
	assert.Equal(t, "cert-a", result.Certificates[0].K8sSecret.Name)
	assert.Equal(t, []string{"base.localhost"}, result.Certificates[0].DNSNames)
}

func TestOverlayCertificate_PartialOverride(t *testing.T) {
	base := domain.CertificateRequest{
		Type:     domain.CertificateTypeServer,
		DNSNames: []string{"base.localhost"},
		K8sSecret: domain.K8sSecretConfig{
			Name: "cert-a",
			Type: domain.K8sSecretTypeTLS,
		},
	}

	overlay := domain.CertificateRequest{
		Type: domain.CertificateTypeClient,
		K8sSecret: domain.K8sSecretConfig{
			Name: "cert-a",
		},
	}

	overlayCertificate(&base, &overlay)

	assert.Equal(t, domain.CertificateTypeClient, base.Type)
	assert.Equal(t, []string{"base.localhost"}, base.DNSNames)    // kept
	assert.Equal(t, domain.K8sSecretTypeTLS, base.K8sSecret.Type) // kept
}

func TestFileSystemConfigRepository_LoadConfig_CachesResult(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Create a valid config file
	configContent := `contexts:
  - name: test-context
    services:
      - name: test-service
        helmRepoPath: /tmp/foo
        helmBranch: main
        helmChartRelativePath: helm
        dockerImages:
          - name: test-image
            dockerfilePath: Dockerfile
            buildContextRelativePath: "."
            gitRepoPath: /tmp/repo
            gitRef: main
`
	configPath := filepath.Join("~", ".pilot-config.yaml")
	err := fs.WriteFile(configPath, []byte(configContent), ports.ReadWrite)
	require.NoError(t, err)

	// First load
	config1, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, config1)

	// Second load should return cached result (same pointer)
	config2, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Same(t, config1, config2, "LoadConfig should return cached result")
}

func TestFileSystemConfigRepository_LoadCurrentConfigurationContext_NotFound(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Set current context to a name that doesn't exist in config
	currentContextPath := filepath.Join("~", ".pilot", "current-context")
	err := fs.WriteFile(currentContextPath, []byte("nonexistent-context"), ports.ReadWrite)
	require.NoError(t, err)

	// Create a config that doesn't contain "nonexistent-context"
	configContent := `contexts:
  - name: existing-context
    services:
      - name: test-service
        helmRepoPath: /tmp/foo
        helmBranch: main
        helmChartRelativePath: helm
        dockerImages:
          - name: test-image
            dockerfilePath: Dockerfile
            buildContextRelativePath: "."
            gitRepoPath: /tmp/repo
            gitRef: main
`
	configPath := filepath.Join("~", ".pilot-config.yaml")
	err = fs.WriteFile(configPath, []byte(configContent), ports.ReadWrite)
	require.NoError(t, err)

	// LoadCurrentConfigurationContext should fail because context doesn't exist
	_, err = repo.LoadCurrentConfigurationContext()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in config")
}

func TestFileSystemConfigRepository_InitConfig_FailsWhenConfigExists(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// Create an existing config file
	configPath := filepath.Join("~", ".pilot-config.yaml")
	err := fs.WriteFile(configPath, []byte("contexts: []"), ports.ReadWrite)
	require.NoError(t, err)

	// InitConfig should fail
	err = repo.InitConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestFileSystemConfigRepository_InitConfig_Success(t *testing.T) {
	fs := testutil.NewTestFileSystem(t)
	repo := NewFileSystemConfigRepository(fs, &mockSecretsRepository{}, &mockTemplater{})

	// InitConfig should succeed when no config exists
	err := repo.InitConfig()
	require.NoError(t, err)

	// Verify config was created
	exists, err := repo.ConfigExists()
	require.NoError(t, err)
	assert.True(t, exists)
}
