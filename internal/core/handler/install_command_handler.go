package handler

import (
	"fmt"
	"os"

	"dx/internal/cli/output"
	"dx/internal/cli/progress"
	"dx/internal/core"
	"dx/internal/ports"
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

func ProvideInstallCommandHandler(
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

	// Dev-proxy is always installed when there are services to install (to apply
	// fresh certificates via Helm), but only rebuilt (Docker image) when the
	// configuration checksum changes.
	totalItems := len(servicesToInstall) + 1

	output.PrintHeader("Installing services")
	fmt.Println()

	// Build names and infos for tracker
	names := make([]string, 0, totalItems)
	infos := make([]string, 0, totalItems)

	names = append(names, "dev-proxy")
	infos = append(infos, "dx")

	for _, svc := range servicesToInstall {
		names = append(names, svc.Name)
		infos = append(infos, "")
	}

	tracker := progress.NewTrackerWithInfoAndVerb(names, infos, "Installing")
	tracker.Start()

	currentIndex := 0
	var devProxyPassword string

	// Install dev-proxy first: rebuild if needed, always install via Helm
	{
		tracker.StartItem(currentIndex)

		if shouldRebuildDevProxy {
			password, err := h.devProxyManager.SaveConfiguration(interceptHttp)
			if err != nil {
				tracker.CompleteItem(currentIndex, err)
				tracker.PrintItemComplete(currentIndex)
				tracker.Stop()
				return err
			}
			devProxyPassword = password

			if err := h.devProxyManager.BuildDevProxy(interceptHttp); err != nil {
				tracker.CompleteItem(currentIndex, err)
				tracker.PrintItemComplete(currentIndex)
				tracker.Stop()
				return err
			}
		}

		if err := h.devProxyManager.InstallDevProxy(devProxyCertSecrets); err != nil {
			tracker.CompleteItem(currentIndex, err)
			tracker.PrintItemComplete(currentIndex)
			tracker.Stop()
			return err
		}

		tracker.CompleteItem(currentIndex, nil)
		tracker.PrintItemComplete(currentIndex)
		currentIndex++
	}

	// Install user services
	for _, service := range servicesToInstall {
		tracker.StartItem(currentIndex)

		err := h.scm.Download(service.HelmRepoPath, service.HelmBranch, service.HelmPath)
		if err != nil {
			tracker.CompleteItem(currentIndex, err)
			tracker.PrintItemComplete(currentIndex)
			tracker.Stop()
			return err
		}

		if err = h.containerOrchestrator.InstallService(&service, renderedCertSecrets[service.Name]); err != nil {
			installErr := fmt.Errorf("failed to install service %s: %v", service.Name, err)
			tracker.CompleteItem(currentIndex, installErr)
			tracker.PrintItemComplete(currentIndex)
			tracker.Stop()
			return installErr
		}

		tracker.CompleteItem(currentIndex, nil)
		tracker.PrintItemComplete(currentIndex)
		currentIndex++
	}

	tracker.Stop()
	fmt.Println()
	output.PrintSuccess(fmt.Sprintf("Installed %d %s", totalItems, output.Plural(totalItems, "service", "services")))

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
