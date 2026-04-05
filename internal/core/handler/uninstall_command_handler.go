package handler

import (
	"fmt"
	"slices"

	"dx/internal/cli/output"
	"dx/internal/cli/progress"
	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/ports"
)

type UninstallCommandHandler struct {
	configRepository      core.ConfigRepository
	containerOrchestrator ports.ContainerOrchestrator
	environmentEnsurer    core.EnvironmentEnsurer
	devProxyManager       *core.DevProxyManager
}

func ProvideUninstallCommandHandler(
	configRepository core.ConfigRepository,
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
	var servicesToUninstall []domain.Service
	for _, service := range configContext.Services {
		if len(services) == 0 && !slices.Contains(service.Profiles, selectedProfile) {
			continue
		}

		if len(services) > 0 && !slices.ContainsFunc(services, func(s string) bool { return s == service.Name }) {
			continue
		}

		servicesToUninstall = append(servicesToUninstall, service)
	}

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
