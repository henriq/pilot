package config_repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pilot/internal/core/domain"
	"pilot/internal/ports"

	"gopkg.in/yaml.v3"
)

var _ ports.ConfigRepository = (*FileSystemConfigRepository)(nil)

var configFilePath = filepath.Join("~", ".pilot-config.yaml")
var currentContextPath = filepath.Join("~", ".pilot", "current-context")

type FileSystemConfigRepository struct {
	fileService       ports.FileSystem
	secretsRepository ports.SecretsRepository
	templater         ports.Templater
	config            *domain.Config
}

func NewFileSystemConfigRepository(
	fileService ports.FileSystem,
	secretsRepository ports.SecretsRepository,
	templater ports.Templater,
) *FileSystemConfigRepository {
	return &FileSystemConfigRepository{
		fileService:       fileService,
		secretsRepository: secretsRepository,
		templater:         templater,
	}
}

// LoadConfig reads, caches, and returns the raw configuration: imports are
// merged, but cache-path derivation, git-ref inheritance, and profile
// defaulting are not applied (that is ResolveContext's job, reached via
// LoadCurrentConfigurationContext).
func (c *FileSystemConfigRepository) LoadConfig() (*domain.Config, error) {
	if c.config != nil {
		return c.config, nil
	}
	// Read the config file
	data, err := c.fileService.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse the YAML
	var config domain.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	home, err := c.fileService.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}

	for i := range config.Contexts {
		context := &config.Contexts[i]
		if context.Import != nil {
			// Import paths are user-specified and may point to any location on the filesystem.
			// This is intentional - users may want to import shared config from project directories.
			// We use os.ReadFile directly since the restricted FileSystem is only for ~/.pilot/ paths.
			importPath := expandImportPath(*context.Import, home)
			data, err := os.ReadFile(importPath) //nolint:gosec // import paths intentionally unrestricted per design
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: failed to read import file %s: %v\n", importPath, err)
			} else {
				var baseContextConfig domain.ConfigurationContext
				if err := yaml.Unmarshal(data, &baseContextConfig); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: failed to parse configuration file %s: %v\n", importPath, err)
				} else {
					config.Contexts[i] = mergeConfigurationContexts(baseContextConfig, *context)
				}
			}
		}
	}

	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %v", err)
	}

	c.config = &config

	return &config, nil
}

func (c *FileSystemConfigRepository) LoadEnvKey(contextName string) (string, error) {
	envKeyPath := filepath.Join("~", ".pilot", contextName, "env-key")

	fileExists, err := c.fileService.FileExists(envKeyPath)
	if err != nil {
		return "", err
	}
	if !fileExists {
		return "", fmt.Errorf("env-key does not exist, see operation 'gen-env-key'")
	}

	data, err := c.fileService.ReadFile(envKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read env key file: %v", err)
	}
	return string(data), nil
}

func (c *FileSystemConfigRepository) SaveConfig(config *domain.Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	return c.fileService.WriteFile(configFilePath, data, ports.ReadWrite)
}

func (c *FileSystemConfigRepository) ConfigExists() (bool, error) {
	return c.fileService.FileExists(configFilePath)
}

func (c *FileSystemConfigRepository) LoadCurrentContextName() (string, error) {
	data, err := c.fileService.ReadFile(currentContextPath)
	if err != nil {
		return "", fmt.Errorf("failed to read current context file: %v", err)
	}
	contextName := strings.TrimSpace(string(data))
	if err := domain.ValidateContextName(contextName); err != nil {
		return "", fmt.Errorf("invalid context name in current-context file: %w", err)
	}
	return contextName, nil
}

func (c *FileSystemConfigRepository) SaveCurrentContextName(currentContextName string) error {
	if err := domain.ValidateContextName(currentContextName); err != nil {
		return fmt.Errorf("invalid context name: %w", err)
	}
	return c.fileService.WriteFile(currentContextPath, []byte(currentContextName), ports.ReadWrite)
}

// LoadCurrentConfigurationContext returns the current context fully resolved:
// imports merged, cache paths derived, git refs inherited, and profiles
// populated.
func (c *FileSystemConfigRepository) LoadCurrentConfigurationContext() (*domain.ConfigurationContext, error) {
	return c.ResolveCurrentConfigurationContext(domain.NoOverrides)
}

// ResolveCurrentConfigurationContext returns the current context resolved with
// the given CLI overrides applied. It derives a fresh copy per call and never
// mutates the cached raw config, so one command's overrides cannot leak into
// the next.
func (c *FileSystemConfigRepository) ResolveCurrentConfigurationContext(
	overrides domain.ContextOverrides,
) (*domain.ConfigurationContext, error) {
	currentContextName, err := c.LoadCurrentContextName()
	if err != nil {
		return nil, err
	}

	config, err := c.LoadConfig()
	if err != nil {
		return nil, err
	}

	home, err := c.fileService.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	for _, context := range config.Contexts {
		if context.Name == currentContextName {
			resolved, err := domain.ResolveContext(context, home, overrides)
			if err != nil {
				return nil, err
			}
			return &resolved, nil
		}
	}

	return nil, fmt.Errorf("current context '%s' not found in config", currentContextName)
}

func (c *FileSystemConfigRepository) InitConfig() error {
	fileExists, err := c.fileService.FileExists(configFilePath)
	if err != nil {
		return err
	}
	if fileExists {
		return fmt.Errorf("configuration file already exists at %s", configFilePath)
	}

	config := domain.CreateDefaultConfig()
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %v", err)
	}

	return c.fileService.WriteFile(configFilePath, data, ports.ReadWrite)
}

// expandImportPath expands ~ to home directory for import paths.
// Import paths can be anywhere on the filesystem, so this is separate from the restricted FileSystem.
func expandImportPath(path string, home string) string {
	// Handle both Unix (~/) and Windows (~\) tilde paths
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		return home
	}
	return path
}

func mergeConfigurationContexts(
	base domain.ConfigurationContext, overlay domain.ConfigurationContext,
) domain.ConfigurationContext {
	if overlay.Name != "" {
		base.Name = overlay.Name
	}

	base.MinPilotVersion = domain.HigherPilotVersion(base.MinPilotVersion, overlay.MinPilotVersion)

	if overlay.Scripts != nil {
		if base.Scripts == nil {
			base.Scripts = make(map[string]string)
		}

		// Add or override scripts from overlay
		for name, script := range overlay.Scripts {
			base.Scripts[name] = script
		}
	}

	for _, overlaySvc := range overlay.Services {
		for i, baseSvc := range base.Services {
			if baseSvc.Name == overlaySvc.Name {
				base.Services[i] = overlayService(base.Services[i], overlaySvc)
			}
		}
	}

	base.LocalServices = append(base.LocalServices, overlay.LocalServices...)
	return base
}

func overlayService(baseService domain.Service, overlayService domain.Service) domain.Service {
	if overlayService.Name != "" {
		baseService.Name = overlayService.Name
	}
	if overlayService.GitRepoPath != "" {
		baseService.GitRepoPath = overlayService.GitRepoPath
	}
	if overlayService.GitRef != "" {
		baseService.GitRef = overlayService.GitRef
	}
	if overlayService.HelmRepoPath != "" {
		baseService.HelmRepoPath = overlayService.HelmRepoPath
	}
	if overlayService.HelmBranch != "" {
		baseService.HelmBranch = overlayService.HelmBranch
	}
	if overlayService.HelmChartRelativePath != "" {
		baseService.HelmChartRelativePath = overlayService.HelmChartRelativePath
	}
	if overlayService.DockerImages != nil {
		for _, overlayImage := range overlayService.DockerImages {
			for i, baseImage := range baseService.DockerImages {
				if baseImage.Name == overlayImage.Name {
					overlayDockerImage(&baseService.DockerImages[i], &overlayImage)
				}
			}
		}
	}
	if overlayService.RemoteImages != nil {
		baseService.RemoteImages = append(baseService.RemoteImages, overlayService.RemoteImages...)
	}
	if overlayService.Certificates != nil {
		for _, overlayCert := range overlayService.Certificates {
			for i, baseCert := range baseService.Certificates {
				if baseCert.K8sSecret.Name == overlayCert.K8sSecret.Name {
					overlayCertificate(&baseService.Certificates[i], &overlayCert)
				}
			}
		}
	}

	return baseService
}

func overlayCertificate(baseCert *domain.CertificateRequest, overlayCert *domain.CertificateRequest) {
	if overlayCert.Type != "" {
		baseCert.Type = overlayCert.Type
	}
	if overlayCert.DNSNames != nil {
		baseCert.DNSNames = overlayCert.DNSNames
	}
	if overlayCert.K8sSecret.Type != "" {
		baseCert.K8sSecret.Type = overlayCert.K8sSecret.Type
	}
	if overlayCert.K8sSecret.Keys != nil {
		baseCert.K8sSecret.Keys = overlayCert.K8sSecret.Keys
	}
}

func overlayDockerImage(baseImage *domain.DockerImage, overlayImage *domain.DockerImage) {
	if overlayImage.Name != "" {
		baseImage.Name = overlayImage.Name
	}
	if overlayImage.GitRepoPath != "" {
		baseImage.GitRepoPath = overlayImage.GitRepoPath
	}
	if overlayImage.GitRef != "" {
		baseImage.GitRef = overlayImage.GitRef
	}
	if overlayImage.DockerfilePath != "" {
		baseImage.DockerfilePath = overlayImage.DockerfilePath
	}
	if overlayImage.DockerfileOverride != "" {
		baseImage.DockerfileOverride = overlayImage.DockerfileOverride
	}
	if overlayImage.BuildArgs != nil {
		baseImage.BuildArgs = append(baseImage.BuildArgs, overlayImage.BuildArgs...)
	}
}
