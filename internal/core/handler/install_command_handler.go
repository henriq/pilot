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
	certsByService, provisionedSecrets, err := h.certificateProvisioner.ProvisionCertificateData(serviceCerts, configContext.Name)
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

	if len(provisionedSecrets) > 0 {
		output.PrintHeader("Provisioning certificates")
		fmt.Println()
		for _, name := range provisionedSecrets {
			fmt.Printf("  %s %s\n", output.SymbolBullet, name)
		}
		output.PrintSuccess(
			fmt.Sprintf(
				"Provisioned %d %s",
				len(provisionedSecrets),
				output.Plural(len(provisionedSecrets), "certificate", "certificates"),
			),
		)
		fmt.Println()
	}

	// Always rebuild dev-proxy when intercepting HTTP so a fresh password is generated.
	// Otherwise, only rebuild when the configuration checksum has changed (including
	// certificate secret changes).
	devProxyCertSecrets := renderedCertSecrets["dev-proxy"]
	shouldRebuildDevProxy := interceptHttp
	if !shouldRebuildDevProxy {
		shouldRebuildDevProxy, err = h.devProxyManager.ShouldRebuildDevProxy(interceptHttp, devProxyCertSecrets)
		if err != nil {
			return err
		}
	}

	totalItems := len(servicesToInstall)
	if shouldRebuildDevProxy {
		totalItems++
	}

	if totalItems == 0 {
		return nil
	}

	output.PrintHeader("Installing services")
	fmt.Println()

	// Build names and infos for tracker
	names := make([]string, 0, totalItems)
	infos := make([]string, 0, totalItems)

	if shouldRebuildDevProxy {
		names = append(names, "dev-proxy")
		infos = append(infos, "dx")
	}

	for _, svc := range servicesToInstall {
		names = append(names, svc.Name)
		infos = append(infos, "")
	}

	tracker := progress.NewTrackerWithInfoAndVerb(names, infos, "Installing")
	tracker.Start()

	currentIndex := 0
	var devProxyPassword string

	// Install dev-proxy first if needed
	if shouldRebuildDevProxy {
		tracker.StartItem(currentIndex)

		password, err := h.devProxyManager.SaveConfiguration(interceptHttp, devProxyCertSecrets)
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
