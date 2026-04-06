package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"dx/internal/core/domain"
	"dx/internal/ports"
)

// DevProxyManager orchestrates dev-proxy operations including configuration saving,
// image building, and service installation/uninstallation.
type DevProxyManager struct {
	configRepository         ports.ConfigRepository
	fileService              ports.FileSystem
	containerImageRepository ports.ContainerImageRepository
	containerOrchestrator    ports.ContainerOrchestrator
	configGenerator          *DevProxyConfigGenerator
}

// ProvideDevProxyManager creates a new DevProxyManager with all required dependencies.
func ProvideDevProxyManager(
	configRepository ports.ConfigRepository,
	fileService ports.FileSystem,
	containerImageRepository ports.ContainerImageRepository,
	containerOrchestrator ports.ContainerOrchestrator,
	configGenerator *DevProxyConfigGenerator,
) *DevProxyManager {
	return &DevProxyManager{
		configRepository:         configRepository,
		fileService:              fileService,
		containerImageRepository: containerImageRepository,
		containerOrchestrator:    containerOrchestrator,
		configGenerator:          configGenerator,
	}
}

// ShouldRebuildDevProxy determines if the dev-proxy needs to be rebuilt.
// Returns true if the dev-proxy doesn't exist or if the configuration has changed.
func (d *DevProxyManager) ShouldRebuildDevProxy(interceptHttp bool) (bool, error) {
	configContext, err := d.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return false, fmt.Errorf("failed to load configuration context: %w", err)
	}

	currentChecksum, err := d.containerOrchestrator.GetDevProxyChecksum()
	if err != nil {
		return false, fmt.Errorf("failed to get current dev-proxy checksum: %w", err)
	}

	// If no checksum exists, dev-proxy needs to be built
	if currentChecksum == "" {
		return true, nil
	}

	newChecksum := d.configGenerator.GenerateChecksum(configContext, interceptHttp)
	return currentChecksum != newChecksum, nil
}

// SaveConfiguration generates and saves all dev-proxy configuration files
// to $HOME/.dx/$CONTEXT_NAME/dev-proxy/
// When interceptHttp is true, mitmproxy configuration is also written.
// Returns the generated mitmweb password when interceptHttp is true, or empty string otherwise.
func (d *DevProxyManager) SaveConfiguration(interceptHttp bool) (string, error) {
	configContext, err := d.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return "", err
	}

	var password string
	if interceptHttp {
		password, err = generateRandomPassword()
		if err != nil {
			return "", err
		}
	}

	configs, err := d.configGenerator.Generate(configContext, interceptHttp, password)
	if err != nil {
		return "", err
	}

	basePath := filepath.Join("~", ".dx", configContext.Name, "dev-proxy")

	// Write HAProxy config
	err = d.fileService.WriteFile(
		filepath.Join(basePath, "haproxy", "haproxy.cfg"),
		configs.HAProxyConfig,
		ports.ReadAllWriteOwner,
	)
	if err != nil {
		return "", err
	}

	// Write HAProxy Dockerfile
	err = d.fileService.WriteFile(
		filepath.Join(basePath, "haproxy", "Dockerfile"),
		configs.HAProxyDockerfile,
		ports.ReadWrite,
	)
	if err != nil {
		return "", err
	}

	// Write mitmproxy Dockerfile only when HTTP interception is enabled;
	// remove any previously written mitmproxy directory when switching back.
	if interceptHttp {
		err = d.fileService.WriteFile(
			filepath.Join(basePath, "mitmproxy", "Dockerfile"),
			configs.MitmProxyDockerfile,
			ports.ReadWrite,
		)
		if err != nil {
			return "", err
		}
	} else {
		if err = d.fileService.RemoveAll(filepath.Join(basePath, "mitmproxy")); err != nil {
			return "", err
		}
	}

	// Write Helm Chart.yaml
	err = d.fileService.WriteFile(
		filepath.Join(basePath, "helm", "Chart.yaml"),
		configs.HelmChartYaml,
		ports.ReadWrite,
	)
	if err != nil {
		return "", err
	}

	// Write Helm deployment manifest
	err = d.fileService.WriteFile(
		filepath.Join(basePath, "helm", "templates", "dev-proxy.yaml"),
		configs.HelmDeploymentYaml,
		ports.ReadWrite,
	)
	if err != nil {
		return "", err
	}

	return configs.Password, nil
}

// BuildDevProxy builds the HAProxy Docker image, and optionally the mitmproxy image
// when HTTP interception is enabled.
func (d *DevProxyManager) BuildDevProxy(interceptHttp bool) error {
	configContext, err := d.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	homeDir, err := d.fileService.HomeDir()
	if err != nil {
		return err
	}
	dockerImages := []domain.DockerImage{
		{
			Name:                     fmt.Sprintf("%s/haproxy-%s", DevProxyImagePrefix, configContext.Name),
			DockerfilePath:           "Dockerfile",
			BuildContextRelativePath: ".",
			Path:                     filepath.Join(homeDir, ".dx", configContext.Name, "dev-proxy", "haproxy"),
		},
	}

	if interceptHttp {
		dockerImages = append(dockerImages, domain.DockerImage{
			Name:                     fmt.Sprintf("%s/mitmproxy-%s", DevProxyImagePrefix, configContext.Name),
			DockerfilePath:           "Dockerfile",
			BuildContextRelativePath: ".",
			Path:                     filepath.Join(homeDir, ".dx", configContext.Name, "dev-proxy", "mitmproxy"),
		})
	}

	for _, image := range dockerImages {
		err = d.containerImageRepository.BuildImage(image)
		if err != nil {
			return fmt.Errorf("failed to build image %s: %w", image.Name, err)
		}
	}

	return nil
}

// InstallDevProxy installs the dev-proxy service to Kubernetes using Helm.
func (d *DevProxyManager) InstallDevProxy(certificateSecrets []byte) error {
	configContext, err := d.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	homeDir, err := d.fileService.HomeDir()
	if err != nil {
		return err
	}

	service := domain.Service{
		Name:     "dev-proxy",
		HelmPath: filepath.Join(homeDir, ".dx", configContext.Name, "dev-proxy", "helm"),
	}
	return d.containerOrchestrator.InstallDevProxy(&service, certificateSecrets)
}

// UninstallDevProxy removes the dev-proxy service from Kubernetes.
func (d *DevProxyManager) UninstallDevProxy() error {
	configContext, err := d.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	homeDir, err := d.fileService.HomeDir()
	if err != nil {
		return err
	}

	service := domain.Service{
		Name:     "dev-proxy",
		HelmPath: filepath.Join(homeDir, ".dx", configContext.Name, "dev-proxy", "helm"),
	}
	return d.containerOrchestrator.UninstallService(&service)
}

func generateRandomPassword() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
