package handler

import (
	"fmt"
	"os"
	"slices"

	"dx/internal/cli/output"
	"dx/internal/cli/progress"
	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/ports"
)

type InstallCommandHandler struct {
	configRepository         core.ConfigRepository
	containerImageRepository ports.ContainerImageRepository
	containerOrchestrator    ports.ContainerOrchestrator
	devProxyManager          *core.DevProxyManager
	environmentEnsurer       core.EnvironmentEnsurer
	scm                      ports.Scm
}

func ProvideInstallCommandHandler(
	configRepository core.ConfigRepository,
	containerImageRepository ports.ContainerImageRepository,
	containerOrchestrator ports.ContainerOrchestrator,
	devProxyManager *core.DevProxyManager,
	environmentEnsurer core.EnvironmentEnsurer,
	scm ports.Scm,
) InstallCommandHandler {
	return InstallCommandHandler{
		configRepository:         configRepository,
		containerImageRepository: containerImageRepository,
		containerOrchestrator:    containerOrchestrator,
		devProxyManager:          devProxyManager,
		environmentEnsurer:       environmentEnsurer,
		scm:                      scm,
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
	var servicesToInstall []domain.Service
	for _, service := range configContext.Services {
		if len(services) == 0 && !slices.Contains(service.Profiles, selectedProfile) {
			continue
		}

		if len(services) > 0 && !slices.ContainsFunc(services, func(s string) bool { return s == service.Name }) {
			continue
		}

		service.InterceptHttp = interceptHttp
		servicesToInstall = append(servicesToInstall, service)
	}

	// Always rebuild dev-proxy when intercepting HTTP so a fresh password is generated.
	// Otherwise, only rebuild when the configuration checksum has changed.
	shouldRebuildDevProxy := interceptHttp
	if !shouldRebuildDevProxy {
		shouldRebuildDevProxy, err = h.devProxyManager.ShouldRebuildDevProxy(interceptHttp)
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

		if err := h.devProxyManager.InstallDevProxy(); err != nil {
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

		if err = h.containerOrchestrator.InstallService(&service); err != nil {
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
		output.PrintInfo(fmt.Sprintf("mitmweb: %s",
			output.Bold(fmt.Sprintf("http://dev-proxy.%s.localhost", configContext.Name))))
		// Intentionally displayed to the user for local dev-proxy access.
		// Uses WriteString to avoid CodeQL go/clear-text-logging false positive.
		os.Stderr.WriteString("  " + output.SymbolArrow + " " + output.Secondary("password: "+devProxyPassword) + "\n") //nolint:errcheck,gosec // intentional stderr output for local dev-proxy password
	}

	return nil
}
