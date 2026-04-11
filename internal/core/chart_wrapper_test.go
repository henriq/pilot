package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"pilot/internal/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chartWrapperMockFileSystem implements ports.FileSystem for testing
type chartWrapperMockFileSystem struct {
	writtenFiles       map[string][]byte
	writtenAccessModes map[string]ports.AccessMode
	createdDirs        map[string]bool
	removedPaths       map[string]bool
	writeError         error
	writeErrorFunc     func(path string) error
	mkdirAllError      error
	removeAllError     error
	homeDirResult      string
	homeDirError       error
}

func newChartWrapperMockFileSystem() *chartWrapperMockFileSystem {
	// Get actual home dir for default behavior
	home, _ := os.UserHomeDir()
	return &chartWrapperMockFileSystem{
		writtenFiles:       make(map[string][]byte),
		writtenAccessModes: make(map[string]ports.AccessMode),
		createdDirs:        make(map[string]bool),
		removedPaths:       make(map[string]bool),
		homeDirResult:      home,
	}
}

func (m *chartWrapperMockFileSystem) WriteFile(path string, content []byte, accessMode ports.AccessMode) error {
	if m.writeErrorFunc != nil {
		if err := m.writeErrorFunc(path); err != nil {
			return err
		}
	}
	if m.writeError != nil {
		return m.writeError
	}
	m.writtenFiles[path] = content
	m.writtenAccessModes[path] = accessMode
	return nil
}

func (m *chartWrapperMockFileSystem) ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func (m *chartWrapperMockFileSystem) FileExists(path string) (bool, error) {
	return false, nil
}

func (m *chartWrapperMockFileSystem) EnsureDirExists(path string) error {
	return nil
}

func (m *chartWrapperMockFileSystem) MkdirAll(path string, accessMode ports.AccessMode) error {
	if m.mkdirAllError != nil {
		return m.mkdirAllError
	}
	m.createdDirs[path] = true
	return nil
}

func (m *chartWrapperMockFileSystem) RemoveAll(path string) error {
	if m.removeAllError != nil {
		return m.removeAllError
	}
	m.removedPaths[path] = true
	return nil
}

func (m *chartWrapperMockFileSystem) HomeDir() (string, error) {
	if m.homeDirError != nil {
		return "", m.homeDirError
	}
	return m.homeDirResult, nil
}

func TestSanitizeName_ValidNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple alphanumeric",
			input:    "myrelease",
			expected: "myrelease",
		},
		{
			name:     "with dashes",
			input:    "my-release",
			expected: "my-release",
		},
		{
			name:     "with underscores",
			input:    "my_release",
			expected: "my_release",
		},
		{
			name:     "mixed case",
			input:    "MyRelease123",
			expected: "MyRelease123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeName_PathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double dot",
			input:    "..",
			expected: "",
		},
		{
			name:     "quadruple dot (bypass attempt)",
			input:    "....",
			expected: "",
		},
		{
			name:     "path traversal sequence",
			input:    "../../../etc/passwd",
			expected: "etcpasswd",
		},
		{
			name:     "mixed path traversal",
			input:    "release/../secret",
			expected: "releasesecret",
		},
		{
			name:     "forward slash",
			input:    "release/name",
			expected: "releasename",
		},
		{
			name:     "backslash",
			input:    "release\\name",
			expected: "releasename",
		},
		{
			name:     "complex bypass attempt",
			input:    "....//....\\\\",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeName_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "spaces",
			input:    "my release",
			expected: "myrelease",
		},
		{
			name:     "special chars",
			input:    "release@#$%^&*()",
			expected: "release",
		},
		{
			name:     "leading dash",
			input:    "-release",
			expected: "release",
		},
		{
			name:     "trailing dash",
			input:    "release-",
			expected: "release",
		},
		{
			name:     "leading underscore",
			input:    "_release",
			expected: "release",
		},
		{
			name:     "only special chars",
			input:    "@#$%",
			expected: "",
		},
		{
			name:     "unicode characters",
			input:    "release日本語",
			expected: "release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChartWrapper_Generate_Success(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:       "test-release",
		ContextName:       "test-context",
		PatchedManifests:  []byte("apiVersion: v1\nkind: ConfigMap\n"),
		OriginalChartName: "original-chart",
		OriginalChartPath: "/path/to/chart",
	}

	path, err := wrapper.Generate(config)
	require.NoError(t, err)

	// Generate returns an absolute path for external tools like Helm
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	expectedAbsPath := filepath.Join(home, ".pilot", "test-context", "wrapper-charts", "test-release")
	assert.Equal(t, expectedAbsPath, path)

	// FileSystem operations use tilde paths internally
	tildePath := filepath.Join("~", ".pilot", "test-context", "wrapper-charts", "test-release")

	// Verify directory was created via MkdirAll
	templatesPath := filepath.Join(tildePath, "templates")
	assert.True(t, fs.createdDirs[templatesPath], "templates directory should be created")

	// Verify Chart.yaml was written
	chartYamlPath := filepath.Join(tildePath, "Chart.yaml")
	assert.Contains(t, fs.writtenFiles, chartYamlPath)
	assert.Contains(t, string(fs.writtenFiles[chartYamlPath]), "name: test-release-wrapper")
	assert.Contains(t, string(fs.writtenFiles[chartYamlPath]), "pilot.wrapped-chart: \"original-chart\"")

	// Verify manifests were written
	manifestsPath := filepath.Join(tildePath, "templates", "manifests.yaml")
	assert.Contains(t, fs.writtenFiles, manifestsPath)
	assert.Equal(t, config.PatchedManifests, fs.writtenFiles[manifestsPath])
}

func TestChartWrapper_Generate_InvalidReleaseName(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:      "..",
		ContextName:      "test-context",
		PatchedManifests: []byte("test"),
	}

	_, err := wrapper.Generate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid release name")
}

func TestChartWrapper_Cleanup_Success(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	err := wrapper.Cleanup("test-context", "test-release")
	require.NoError(t, err)

	expectedPath := filepath.Join("~", ".pilot", "test-context", "wrapper-charts", "test-release")
	assert.True(t, fs.removedPaths[expectedPath], "wrapper chart directory should be removed")
}

func TestChartWrapper_Cleanup_InvalidReleaseName(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	err := wrapper.Cleanup("test-context", "..")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid release name")
}

func TestChartWrapper_Generate_MkdirAllError(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	fs.mkdirAllError = errors.New("permission denied")
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:      "test-release",
		ContextName:      "test-context",
		PatchedManifests: []byte("test"),
	}

	_, err := wrapper.Generate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create wrapper chart directory")
}

func TestChartWrapper_Generate_WriteChartYamlError(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	// Fail on Chart.yaml write
	fs.writeErrorFunc = func(path string) error {
		if filepath.Base(path) == "Chart.yaml" {
			return errors.New("disk full")
		}
		return nil
	}
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:      "test-release",
		ContextName:      "test-context",
		PatchedManifests: []byte("test"),
	}

	_, err := wrapper.Generate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write Chart.yaml")
}

func TestChartWrapper_Generate_WriteManifestsError(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	// Fail on manifests.yaml write
	fs.writeErrorFunc = func(path string) error {
		if filepath.Base(path) == "manifests.yaml" {
			return errors.New("disk full")
		}
		return nil
	}
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:      "test-release",
		ContextName:      "test-context",
		PatchedManifests: []byte("test"),
	}

	_, err := wrapper.Generate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write manifests")
}

func TestChartWrapper_Generate_HomeDirError(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	fs.homeDirError = errors.New("cannot determine home directory")
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:      "test-release",
		ContextName:      "test-context",
		PatchedManifests: []byte("test"),
	}

	_, err := wrapper.Generate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine home directory")
}

func TestChartWrapper_Cleanup_RemoveAllError(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	fs.removeAllError = errors.New("directory not empty")
	wrapper := NewChartWrapper(fs)

	// Cleanup is best-effort — RemoveAll errors are ignored
	err := wrapper.Cleanup("test-context", "test-release")
	assert.NoError(t, err)
}

func TestChartWrapper_Generate_InvalidContextName(t *testing.T) {
	// Context names that result in empty string after sanitization should error
	tests := []struct {
		name        string
		contextName string
	}{
		{"double dot only", ".."},
		{"only special chars", "@#$%^&*()"},
		{"only slashes", "//\\\\"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newChartWrapperMockFileSystem()
			wrapper := NewChartWrapper(fs)

			config := WrapperChartConfig{
				ReleaseName:      "valid-release",
				ContextName:      tt.contextName,
				PatchedManifests: []byte("test"),
			}

			_, err := wrapper.Generate(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid context name")
		})
	}
}

func TestChartWrapper_Generate_SanitizesContextName(t *testing.T) {
	// Context names with dangerous chars should be sanitized, not rejected
	tests := []struct {
		name            string
		contextName     string
		expectedContext string
	}{
		{"path traversal", "../etc", "etc"},
		{"forward slash", "foo/bar", "foobar"},
		{"backslash", "foo\\bar", "foobar"},
		{"null byte", "foo\x00bar", "foobar"},
	}

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newChartWrapperMockFileSystem()
			wrapper := NewChartWrapper(fs)

			config := WrapperChartConfig{
				ReleaseName:      "valid-release",
				ContextName:      tt.contextName,
				PatchedManifests: []byte("test"),
			}

			path, err := wrapper.Generate(config)
			require.NoError(t, err)
			// Verify the sanitized context name is used in the returned absolute path
			expectedPath := filepath.Join(home, ".pilot", tt.expectedContext, "wrapper-charts", "valid-release")
			assert.Equal(t, expectedPath, path)
		})
	}
}

func TestChartWrapper_Cleanup_InvalidContextName(t *testing.T) {
	// Context names that result in empty string after sanitization should error
	tests := []struct {
		name        string
		contextName string
	}{
		{"double dot only", ".."},
		{"only special chars", "@#$%^&*()"},
		{"only slashes", "//\\\\"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newChartWrapperMockFileSystem()
			wrapper := NewChartWrapper(fs)

			err := wrapper.Cleanup(tt.contextName, "valid-release")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid context name")
		})
	}
}

func TestChartWrapper_Generate_ManifestsPermissions(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:      "test-release",
		ContextName:      "test-context",
		PatchedManifests: []byte("apiVersion: v1\nkind: Secret\n"),
	}

	_, err := wrapper.Generate(config)
	require.NoError(t, err)

	tildePath := filepath.Join("~", ".pilot", "test-context", "wrapper-charts", "test-release")

	manifestsPath := filepath.Join(tildePath, "templates", "manifests.yaml")
	assert.Equal(t, ports.AccessMode(ports.ReadWrite), fs.writtenAccessModes[manifestsPath], "manifests.yaml should be owner-only readable")

	chartYamlPath := filepath.Join(tildePath, "Chart.yaml")
	assert.Equal(t, ports.AccessMode(ports.ReadAllWriteOwner), fs.writtenAccessModes[chartYamlPath], "Chart.yaml uses standard permissions (no secrets)")
}

func TestGenerateChartYaml_WithAnnotations(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName:       "test",
		OriginalChartName: "original",
		OriginalChartPath: "/path/to/chart",
	}

	yaml := wrapper.generateChartYaml(config)

	assert.Contains(t, yaml, "apiVersion: v2")
	assert.Contains(t, yaml, "name: test-wrapper")
	assert.Contains(t, yaml, "annotations:")
	assert.Contains(t, yaml, "pilot.wrapped-chart: \"original\"")
	assert.Contains(t, yaml, "pilot.wrapped-path: \"/path/to/chart\"")
}

func TestEscapeYamlString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple string", "hello", "hello"},
		{"with quotes", `say "hello"`, `say \"hello\"`},
		{"with backslash", `path\to\file`, `path\\to\\file`},
		{"with newline", "line1\nline2", `line1\nline2`},
		{"with carriage return", "line1\rline2", `line1\rline2`},
		{"with tab", "col1\tcol2", `col1\tcol2`},
		{"mixed special chars", "test\"\n\\", `test\"\n\\`},
		{"yaml injection attempt", "value\"\nmalicious: injected", `value\"\nmalicious: injected`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeYamlString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateChartYaml_EscapesSpecialChars(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	// Attempt YAML injection through OriginalChartPath
	config := WrapperChartConfig{
		ReleaseName:       "test",
		OriginalChartName: "chart-with-\"quotes\"",
		OriginalChartPath: "/path/with\nnewline",
	}

	yaml := wrapper.generateChartYaml(config)

	// Should contain escaped values, not raw injection
	assert.Contains(t, yaml, `pilot.wrapped-chart: "chart-with-\"quotes\""`)
	assert.Contains(t, yaml, `pilot.wrapped-path: "/path/with\nnewline"`)
	// Should NOT contain unescaped newline that would break YAML structure
	assert.NotContains(t, yaml, "pilot.wrapped-path: \"/path/with\n")
}

func TestGenerateChartYaml_WithoutAnnotations(t *testing.T) {
	fs := newChartWrapperMockFileSystem()
	wrapper := NewChartWrapper(fs)

	config := WrapperChartConfig{
		ReleaseName: "test",
	}

	yaml := wrapper.generateChartYaml(config)

	assert.Contains(t, yaml, "apiVersion: v2")
	assert.Contains(t, yaml, "name: test-wrapper")
	assert.NotContains(t, yaml, "annotations:")
}

func TestExpandTildePath(t *testing.T) {
	home := "/home/testuser"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde only",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde with slash",
			input:    "~/path/to/file",
			expected: filepath.Join(home, "path/to/file"),
		},
		{
			name:     "tilde from filepath.Join",
			input:    filepath.Join("~", ".pilot", "context"),
			expected: filepath.Join(home, ".pilot", "context"),
		},
		{
			name:     "absolute path unchanged",
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path unchanged",
			input:    "relative/path",
			expected: "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTildePath(tt.input, home)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChartWrapper_Generate_WritesCertificateSecrets(t *testing.T) {
	mockFS := newChartWrapperMockFileSystem()
	sut := NewChartWrapper(mockFS)

	certSecrets := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: foo-tls\n")

	_, err := sut.Generate(WrapperChartConfig{
		ReleaseName:        "my-service",
		ContextName:        "my-context",
		PatchedManifests:   []byte("manifests"),
		CertificateSecrets: certSecrets,
		OriginalChartName:  "my-service",
	})

	require.NoError(t, err)

	secretsPath := filepath.Join("~", ".pilot", "my-context", "wrapper-charts", "my-service", "templates", "secrets.yaml")
	assert.Equal(t, certSecrets, mockFS.writtenFiles[secretsPath])
}

func TestChartWrapper_Generate_SkipsCertificateSecretsWhenEmpty(t *testing.T) {
	mockFS := newChartWrapperMockFileSystem()
	sut := NewChartWrapper(mockFS)

	_, err := sut.Generate(WrapperChartConfig{
		ReleaseName:      "my-service",
		ContextName:      "my-context",
		PatchedManifests: []byte("manifests"),
	})

	require.NoError(t, err)

	secretsPath := filepath.Join("~", ".pilot", "my-context", "wrapper-charts", "my-service", "templates", "secrets.yaml")
	_, exists := mockFS.writtenFiles[secretsPath]
	assert.False(t, exists, "secrets.yaml should not be written when CertificateSecrets is empty")
}

func TestChartWrapper_Generate_RemovesStaleSecretsYaml(t *testing.T) {
	mockFS := newChartWrapperMockFileSystem()
	sut := NewChartWrapper(mockFS)

	secretsPath := filepath.Join("~", ".pilot", "my-context", "wrapper-charts", "my-service", "templates", "secrets.yaml")

	// First generate with certificate secrets
	_, err := sut.Generate(WrapperChartConfig{
		ReleaseName:        "my-service",
		ContextName:        "my-context",
		PatchedManifests:   []byte("manifests"),
		CertificateSecrets: []byte("secret-yaml"),
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("secret-yaml"), mockFS.writtenFiles[secretsPath])

	// Regenerate without certificate secrets — stale file should be removed
	_, err = sut.Generate(WrapperChartConfig{
		ReleaseName:      "my-service",
		ContextName:      "my-context",
		PatchedManifests: []byte("manifests"),
	})
	require.NoError(t, err)

	assert.True(t, mockFS.removedPaths[secretsPath], "stale secrets.yaml should be removed")
}
