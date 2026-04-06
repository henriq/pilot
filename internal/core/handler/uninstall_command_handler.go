package handler

import (
	"fmt"

	"dx/internal/cli/output"
	"dx/internal/cli/progress"
	"dx/internal/core"
	"dx/internal/ports"
)

type UninstallCommandHandler struct {
	configRepository      ports.ConfigRepository
	containerOrchestrator ports.ContainerOrchestrator
	environmentEnsurer    core.EnvironmentEnsurer
	devProxyManager       *core.DevProxyManager
}

func ProvideUninstallCommandHandler(
	configRepository ports.ConfigRepository,
	containerOrchestrator ports.ContainerOrchestrator,
	environmentEnsurer core.EnvironmentEnsurer,
	devProxyManager *core.DevProxyManager,
) UninstallCommandHandler {
	return UninstallCommandHandler{
		configRepository:      configRepository,
		containerOrchestrator: containerOrchestrator,
		environmentEnsurer:    environmentEnsurer,
		devProxyManager:       devProxyManager,
	}
}

func (h *UninstallCommandHandler) Handle(services []string, selectedProfile string) error {
	err := h.environmentEnsurer.EnsureExpectedClusterIsSelected()
	if err != nil {
		return err
	}

	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	// Collect services to uninstall
	servicesToUninstall := configContext.FilterServices(services, selectedProfile)

	if len(servicesToUninstall) > 0 {
		output.PrintHeader("Uninstalling services")
		fmt.Println()

		serviceNames := make([]string, len(servicesToUninstall))
		for i, svc := range servicesToUninstall {
			serviceNames[i] = svc.Name
		}
		tracker := progress.NewTrackerWithVerb(serviceNames, "Uninstalling")
		tracker.Start()

		for i, service := range servicesToUninstall {
			tracker.StartItem(i)
			err := h.containerOrchestrator.UninstallService(&service)
			tracker.CompleteItem(i, err)
			tracker.PrintItemComplete(i)
		}

		tracker.Stop()
		fmt.Println()
		output.PrintSuccess(fmt.Sprintf("Uninstalled %d %s", len(servicesToUninstall), output.Plural(len(servicesToUninstall), "service", "services")))
	}

	hasDeployedServices, err := h.containerOrchestrator.HasDeployedServices()
	if err != nil {
		return err
	}
	if !hasDeployedServices {
		fmt.Println()
		output.PrintStep("Removing dev-proxy (no services remaining)")
		err = h.devProxyManager.UninstallDevProxy()
		if err != nil {
			return err
		}
		output.PrintSuccess("Removed dev-proxy")
	}

	return nil
}
