package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"dx/internal/ports"
)

// WrapperChartConfig contains configuration for generating a wrapper chart.
type WrapperChartConfig struct {
	ReleaseName       string
	ContextName       string
	PatchedManifests  []byte
	OriginalChartName string
	OriginalChartPath string
}

// ChartWrapper generates wrapper Helm charts containing patched manifests.
type ChartWrapper struct {
	fileSystem ports.FileSystem
}

// ProvideChartWrapper creates a new ChartWrapper.
func ProvideChartWrapper(fileSystem ports.FileSystem) *ChartWrapper {
	return &ChartWrapper{
		fileSystem: fileSystem,
	}
}

// Generate creates a wrapper chart containing the patched manifests.
// Returns the absolute path to the generated wrapper chart.
func (c *ChartWrapper) Generate(config WrapperChartConfig) (string, error) {
	// Sanitize release name
	safeName := sanitizeName(config.ReleaseName)
	if safeName == "" {
		return "", fmt.Errorf("invalid release name: %s", config.ReleaseName)
	}

	// Sanitize context name to prevent path traversal
	safeContext := sanitizeName(config.ContextName)
	if safeContext == "" {
		return "", fmt.Errorf("invalid context name: %s", config.ContextName)
	}

	basePath := filepath.Join("~", ".dx", safeContext, "wrapper-charts", safeName)

	templatesPath := filepath.Join(basePath, "templates")
	if err := c.fileSystem.MkdirAll(templatesPath, ports.ReadWriteExecute); err != nil {
		return "", fmt.Errorf("failed to create wrapper chart directory: %w", err)
	}

	// Generate Chart.yaml
	chartYaml := c.generateChartYaml(config)
	if err := c.fileSystem.WriteFile(
		filepath.Join(basePath, "Chart.yaml"),
		[]byte(chartYaml),
		ports.ReadAllWriteOwner,
	); err != nil {
		return "", fmt.Errorf("failed to write Chart.yaml: %w", err)
	}

	// Write patched manifests
	if err := c.fileSystem.WriteFile(
		filepath.Join(templatesPath, "manifests.yaml"),
		config.PatchedManifests,
		ports.ReadWrite,
	); err != nil {
		return "", fmt.Errorf("failed to write manifests: %w", err)
	}

	// Return absolute path for external tools like Helm
	homeDir, err := c.fileSystem.HomeDir()
	if err != nil {
		return "", err
	}
	return expandTildePath(basePath, homeDir)
}

// Cleanup removes the wrapper chart directory for a release.
func (c *ChartWrapper) Cleanup(contextName, releaseName string) error {
	safeName := sanitizeName(releaseName)
	if safeName == "" {
		return fmt.Errorf("invalid release name: %s", releaseName)
	}

	safeContext := sanitizeName(contextName)
	if safeContext == "" {
		return fmt.Errorf("invalid context name: %s", contextName)
	}

	_ = c.fileSystem.RemoveAll(filepath.Join("~", ".dx", safeContext, "wrapper-charts", safeName))

	return nil
}

// generateChartYaml creates the Chart.yaml content with metadata.
func (c *ChartWrapper) generateChartYaml(config WrapperChartConfig) string {
	var sb strings.Builder

	sb.WriteString("apiVersion: v2\n")
	fmt.Fprintf(&sb, "name: %s-wrapper\n", config.ReleaseName)
	fmt.Fprintf(&sb, "description: Wrapper chart for %s with dx patches applied\n", config.ReleaseName)
	sb.WriteString("type: application\n")
	sb.WriteString("version: 1.0.0\n")
	sb.WriteString("appVersion: \"1.0.0\"\n")

	// Add annotations for traceability
	if config.OriginalChartName != "" || config.OriginalChartPath != "" {
		sb.WriteString("annotations:\n")
		if config.OriginalChartName != "" {
			fmt.Fprintf(&sb, "  dx.wrapped-chart: \"%s\"\n", escapeYamlString(config.OriginalChartName))
		}
		if config.OriginalChartPath != "" {
			fmt.Fprintf(&sb, "  dx.wrapped-path: \"%s\"\n", escapeYamlString(config.OriginalChartPath))
		}
	}

	return sb.String()
}

// escapeYamlString escapes special characters for use in a YAML double-quoted string.
func escapeYamlString(s string) string {
	// Escape backslashes first (must be done before other escapes)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// Escape double quotes
	s = strings.ReplaceAll(s, "\"", "\\\"")
	// Escape newlines
	s = strings.ReplaceAll(s, "\n", "\\n")
	// Escape carriage returns
	s = strings.ReplaceAll(s, "\r", "\\r")
	// Escape tabs
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// expandTildePath expands ~ to the user's home directory using the provided home directory.
// Returns the path unchanged if it doesn't start with ~.
func expandTildePath(path, homeDir string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	if path == "~" {
		return homeDir, nil
	}

	// Handle ~/... paths
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return filepath.Join(homeDir, path[2:]), nil
	}

	// Handle paths like "~/.dx" that use filepath.Join("~", ".dx")
	// which produces "~/.dx" on Unix or "~\.dx" on Windows
	return filepath.Join(homeDir, path[1:]), nil
}

// sanitizeName removes dangerous characters from names (context name, release name, etc.).
// Returns empty string if the name is entirely invalid.
func sanitizeName(name string) string {
	// Remove path traversal attempts - loop until no more ".." sequences exist
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", "")
	}
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	// Remove null bytes
	name = strings.ReplaceAll(name, "\x00", "")

	// Only allow alphanumeric, dash, and underscore
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	name = re.ReplaceAllString(name, "")

	// Trim leading/trailing dashes and underscores
	name = strings.Trim(name, "-_")

	return name
}
