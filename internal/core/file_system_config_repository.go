package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dx/internal/core/domain"
	"dx/internal/ports"

	"gopkg.in/yaml.v3"
)

var configFilePath = filepath.Join("~", ".dx-config.yaml")
var currentContextPath = filepath.Join("~", ".dx", "current-context")

type ConfigRepository interface {
	LoadConfig() (*domain.Config, error)
	SaveConfig(*domain.Config) error
	ConfigExists() (bool, error)
	LoadCurrentConfigurationContext() (*domain.ConfigurationContext, error)
	LoadCurrentContextName() (string, error)
	SaveCurrentContextName(string) error
	LoadEnvKey(contextName string) (string, error)
}

type FileSystemConfigRepository struct {
	fileService       ports.FileSystem
	secretsRepository SecretsRepository
	templater         ports.Templater
	config            *domain.Config
}

func ProvideFileSystemConfigRepository(
	fileService ports.FileSystem,
	secretsRepository SecretsRepository,
	templater ports.Templater,
) *FileSystemConfigRepository {
	return &FileSystemConfigRepository{
		fileService:       fileService,
		secretsRepository: secretsRepository,
		templater:         templater,
	}
}

func CreateTemplatingValues(
	configRepository ConfigRepository,
	secretsRepository SecretsRepository,
) (map[string]interface{}, error) {
	contextName, err := configRepository.LoadCurrentContextName()
	if err != nil {
		return nil, err
	}
	configContext, err := configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, err
	}
	secrets, err := secretsRepository.LoadSecrets(contextName)
	if err != nil {
		return nil, err
	}

	secretsMap := createSecretsMap(secrets)
	servicesMap := createServicesMap(configContext)
	values := map[string]interface{}{
		"Secrets":  secretsMap,
		"Services": servicesMap,
	}
	return values, nil
}

func createServicesMap(configContext *domain.ConfigurationContext) map[string]interface{} {
	servicesMap := make(map[string]interface{})
	for _, service := range configContext.Services {
		serviceHasValue := false
		serviceMap := make(map[string]interface{})
		if service.Path != "" {
			// Convert backslashes to forward slashes so Windows paths are not
			// misinterpreted as escape sequences when used in bash scripts.
			serviceMap["path"] = strings.ReplaceAll(service.Path, "\\", "/")
			serviceHasValue = true
		}
		if service.GitRef != "" {
			serviceMap["gitRef"] = service.GitRef
			serviceHasValue = true
		}

		if serviceHasValue {
			servicesMap[service.Name] = serviceMap
		}
	}

	return servicesMap
}

// Create secrets map, splitting strings by "." to create nested maps
func createSecretsMap(secrets []*domain.Secret) map[string]interface{} {
	secretMap := make(map[string]interface{})
	for _, secret := range secrets {
		parts := strings.Split(secret.Key, ".")
		currentMap := secretMap
		for i, part := range parts {
			if i == len(parts)-1 {
				currentMap[part] = secret.Value
			} else {
				if currentMap[part] == nil {
					currentMap[part] = make(map[string]interface{})
				}
				if nested, ok := currentMap[part].(map[string]interface{}); ok {
					currentMap = nested
				} else {
					// Key conflict: a value already exists at this path but isn't a map.
					// Skip this secret to avoid overwriting the existing value.
					break
				}
			}
		}
	}
	return secretMap
}

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

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}

	for i := range config.Contexts {
		context := &config.Contexts[i]
		if context.Import != nil {
			// Import paths are user-specified and may point to any location on the filesystem.
			// This is intentional - users may want to import shared config from project directories.
			// We use os.ReadFile directly since the restricted FileSystem is only for ~/.dx/ paths.
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

		for j := range context.Services {
			service := &context.Services[j]
			if len(service.Profiles) == 0 {
				service.Profiles = []string{"default"}
			}
			if !slices.Contains(service.Profiles, "all") {
				service.Profiles = append(service.Profiles, "all")
			}
			hasher := sha256.New()
			fmt.Fprintf(hasher, "%s-%s", service.HelmRepoPath, service.HelmBranch)
			hashedName := fmt.Sprintf("%x", hasher.Sum(nil))[:12]
			service.HelmPath = filepath.Join(home, ".dx", context.Name, "charts", hashedName)
			for k := range context.Services[j].DockerImages {
				image := &config.Contexts[i].Services[j].DockerImages[k]
				if image.GitRepoPath == "" {
					image.GitRepoPath = service.GitRepoPath
				}
				if image.GitRef == "" {
					image.GitRef = service.GitRef
				}

				hasher := sha256.New()
				fmt.Fprintf(hasher, "%s-%s", image.GitRepoPath, image.GitRef)
				hashedName = fmt.Sprintf("%x", hasher.Sum(nil))[:12]
				image.Path = filepath.Join(home, ".dx", context.Name, service.Name, hashedName)
			}
			if service.GitRepoPath != "" && service.GitRef != "" {
				hasher := sha256.New()
				fmt.Fprintf(hasher, "%s-%s", service.GitRepoPath, service.GitRef)
				hashedName = fmt.Sprintf("%x", hasher.Sum(nil))[:12]
				service.Path = filepath.Join(home, ".dx", context.Name, service.Name, hashedName)
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
	envKeyPath := filepath.Join("~", ".dx", contextName, "env-key")

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
	if err := validateContextName(contextName); err != nil {
		return "", fmt.Errorf("invalid context name in current-context file: %w", err)
	}
	return contextName, nil
}

func (c *FileSystemConfigRepository) SaveCurrentContextName(currentContextName string) error {
	if err := validateContextName(currentContextName); err != nil {
		return fmt.Errorf("invalid context name: %w", err)
	}
	return c.fileService.WriteFile(currentContextPath, []byte(currentContextName), ports.ReadWrite)
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

// validateContextName checks that a context name doesn't contain path traversal characters.
func validateContextName(name string) error {
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}
	if strings.Contains(name, "..") ||
		strings.Contains(name, "/") ||
		strings.Contains(name, "\\") ||
		strings.Contains(name, "\x00") {
		return fmt.Errorf("context name contains invalid characters")
	}
	return nil
}

func (c *FileSystemConfigRepository) LoadCurrentConfigurationContext() (*domain.ConfigurationContext, error) {
	currentContextName, err := c.LoadCurrentContextName()
	if err != nil {
		return nil, err
	}

	config, err := c.LoadConfig()
	if err != nil {
		return nil, err
	}

	for _, context := range config.Contexts {
		if context.Name == currentContextName {
			return &context, nil
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

func mergeConfigurationContexts(
	base domain.ConfigurationContext, overlay domain.ConfigurationContext,
) domain.ConfigurationContext {
	if overlay.Name != "" {
		base.Name = overlay.Name
	}

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
