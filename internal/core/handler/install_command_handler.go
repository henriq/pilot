package handler

import (
	"fmt"
	"os"

	"pilot/internal/cli/output"
	"pilot/internal/cli/progress"
	"pilot/internal/core"
	"pilot/internal/ports"
)

type InstallCommandHandler struct {
	configRepository         ports.ConfigRepository
	containerImageRepository ports.ContainerImageRepository
	containerOrchestrator    ports.ContainerOrchestrator
	devProxyManager          *core.DevProxyManager
	environmentEnsurer       core.EnvironmentEnsurer
	scm                      ports.Scm
	certificateProvisioner   *core.CertificateProvisioner
}

func NewInstallCommandHandler(
	configRepository ports.ConfigRepository,
	containerImageRepository ports.ContainerImageRepository,
	containerOrchestrator ports.ContainerOrchestrator,
	devProxyManager *core.DevProxyManager,
	environmentEnsurer core.EnvironmentEnsurer,
	scm ports.Scm,
	certificateProvisioner *core.CertificateProvisioner,
) InstallCommandHandler {
	return InstallCommandHandler{
		configRepository:         configRepository,
		containerImageRepository: containerImageRepository,
		containerOrchestrator:    containerOrchestrator,
		devProxyManager:          devProxyManager,
		environmentEnsurer:       environmentEnsurer,
		scm:                      scm,
		certificateProvisioner:   certificateProvisioner,
	}
}

func (h *InstallCommandHandler) Handle(services []string, selectedProfile string, interceptHttp bool) error {
	err := h.environmentEnsurer.EnsureExpectedClusterIsSelected()
	if err != nil {
		return err
	}

	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	// Collect services to install
	servicesToInstall := configContext.FilterServices(services, selectedProfile)
	for i := range servicesToInstall {
		servicesToInstall[i].InterceptHttp = interceptHttp
	}

	// Provision certificate data for services that need them (includes internal TLS for dev-proxy)
	serviceCerts := core.CollectAllCertificates(servicesToInstall, configContext)
	certsByService, err := h.certificateProvisioner.ProvisionCertificateData(serviceCerts, configContext.Name)
	if err != nil {
		return fmt.Errorf("failed to provision certificates: %w", err)
	}

	// Render certificate data as K8s Secret YAML for inclusion in wrapper charts
	renderedCertSecrets := make(map[string][]byte)
	for serviceName, certs := range certsByService {
		rendered, err := core.RenderCertificateSecretManifests(certs)
		if err != nil {
			return fmt.Errorf("failed to render certificate secrets for %s: %w", serviceName, err)
		}
		renderedCertSecrets[serviceName] = rendered
	}

	// Always rebuild dev-proxy when intercepting HTTP so a fresh password is generated.
	// Otherwise, only rebuild when the configuration checksum has changed.
	devProxyCertSecrets := renderedCertSecrets["dev-proxy"]
	shouldRebuildDevProxy := interceptHttp
	if !shouldRebuildDevProxy {
		shouldRebuildDevProxy, err = h.devProxyManager.ShouldRebuildDevProxy(interceptHttp)
		if err != nil {
			return err
		}
	}

	if len(servicesToInstall) == 0 && !shouldRebuildDevProxy {
		return nil
	}

	output.PrintHeader("Installing services")
	fmt.Println()

	// Configure dev-proxy: rebuild if needed, always install via Helm to
	// deliver fresh certificate secrets
	var devProxyPassword string
	if shouldRebuildDevProxy {
		output.PrintStep("Configuring dev-proxy...")

		password, err := h.devProxyManager.SaveConfiguration(interceptHttp)
		if err != nil {
			return err
		}
		devProxyPassword = password

		if err := h.devProxyManager.BuildDevProxy(interceptHttp); err != nil {
			return err
		}
	}

	if err := h.devProxyManager.InstallDevProxy(devProxyCertSecrets); err != nil {
		return err
	}

	// Install user services
	if len(servicesToInstall) > 0 {
		if shouldRebuildDevProxy {
			fmt.Println()
		}

		names := make([]string, 0, len(servicesToInstall))
		for _, svc := range servicesToInstall {
			names = append(names, svc.Name)
		}

		tracker := progress.NewTrackerWithVerb(names, "Installing")
		tracker.Start()

		for i, service := range servicesToInstall {
			tracker.StartItem(i)

			err := h.scm.Download(service.HelmRepoPath, service.HelmBranch, service.HelmPath)
			if err != nil {
				tracker.CompleteItem(i, err)
				tracker.PrintItemComplete(i)
				tracker.Stop()
				return err
			}

			if err = h.containerOrchestrator.InstallService(&service, renderedCertSecrets[service.Name]); err != nil {
				installErr := fmt.Errorf("failed to install service %s: %v", service.Name, err)
				tracker.CompleteItem(i, installErr)
				tracker.PrintItemComplete(i)
				tracker.Stop()
				return installErr
			}

			tracker.CompleteItem(i, nil)
			tracker.PrintItemComplete(i)
		}

		tracker.Stop()
		fmt.Println()
		serviceCount := len(servicesToInstall)
		output.PrintSuccess(fmt.Sprintf("Installed %d %s", serviceCount, output.Plural(serviceCount, "service", "services")))
	}

	if devProxyPassword != "" {
		fmt.Println()
		output.PrintInfo(
			fmt.Sprintf(
				"mitmweb: %s",
				output.Bold(fmt.Sprintf("https://dev-proxy.%s.localhost", configContext.Name)),
			),
		)
		// Intentionally displayed to the user for local dev-proxy access.
		// Uses WriteString to avoid CodeQL go/clear-text-logging false positive.
		_, err = os.Stderr.WriteString("  " + output.SymbolArrow + " " + output.Secondary("password: "+devProxyPassword) + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}
