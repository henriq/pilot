package handler

import (
	"fmt"
	"sort"
	"time"

	"pilot/internal/cli/output"
	"pilot/internal/cli/progress"
	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

type RunCommandHandler struct {
	configRepository  ports.ConfigRepository
	secretsRepository ports.SecretsRepository
	templater         ports.Templater
	scm               ports.Scm
	commandRunner     ports.CommandRunner
}

func NewRunCommandHandler(
	configRepository ports.ConfigRepository,
	secretsRepository ports.SecretsRepository,
	templater ports.Templater,
	scm ports.Scm,
	commandRunner ports.CommandRunner,
) RunCommandHandler {
	return RunCommandHandler{
		configRepository:  configRepository,
		secretsRepository: secretsRepository,
		templater:         templater,
		scm:               scm,
		commandRunner:     commandRunner,
	}
}

func (h *RunCommandHandler) Handle(scripts map[string]string, executionPlan []string) error {
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	renderValues, err := core.CreateTemplatingValues(configContext, h.secretsRepository)
	if err != nil {
		return err
	}

	for _, scriptName := range executionPlan {
		script := scripts[scriptName]

		dependentServices, err := findServiceDependencies(script, configContext.Services)
		if err != nil {
			return err
		}

		if len(dependentServices) > 0 {
			sort.Slice(dependentServices, func(i, j int) bool {
				return dependentServices[i].Name < dependentServices[j].Name
			})

			for _, dependentService := range dependentServices {
				if dependentService.GitRepoPath == "" || dependentService.GitRef == "" {
					return fmt.Errorf("git repository path or ref is empty for service '%s'", dependentService.Name)
				}
			}

			names := make([]string, len(dependentServices))
			refs := make([]string, len(dependentServices))
			for i, service := range dependentServices {
				names[i] = service.Name
				refs[i] = service.GitRef
			}

			fetchStartTime := time.Now()
			output.PrintHeader(fmt.Sprintf("Fetching dependencies for %s", output.Bold(scriptName)))
			fmt.Println()

			tracker := progress.NewTrackerWithInfoAndVerb(names, refs, "Fetching")
			tracker.Start()

			var fetchErr error
			for i, dependentService := range dependentServices {
				tracker.StartItem(i)
				downloadErr := h.scm.Download(dependentService.GitRepoPath, dependentService.GitRef, dependentService.Path)
				tracker.CompleteItem(i, downloadErr)
				tracker.PrintItemComplete(i)
				if downloadErr != nil {
					fetchErr = downloadErr
					break
				}
			}

			tracker.Stop()

			if fetchErr != nil {
				return fetchErr
			}

			fmt.Println()
			output.PrintSuccess(
				fmt.Sprintf(
					"Fetched %d %s in %s",
					len(dependentServices),
					output.Plural(len(dependentServices), "dependency", "dependencies"),
					progress.FormatDuration(time.Since(fetchStartTime)),
				),
			)
			fmt.Println()
		}

		renderedScript, err := h.templater.Render(script, scriptName, renderValues)
		if err != nil {
			return err
		}

		output.PrintStep(fmt.Sprintf("Running %s", output.Bold(scriptName)))
		fmt.Println()

		shell, shellArg := getShellCommand()
		if err := h.commandRunner.RunInteractive(shell, shellArg, renderedScript); err != nil {
			fmt.Println()
			return fmt.Errorf("script '%s' failed: %v", scriptName, err)
		}

		fmt.Println()
	}
	return nil
}

func findServiceDependencies(script string, existingServices []domain.Service) ([]domain.Service, error) {
	if core.ReferencesAllServices(script) {
		services := make([]domain.Service, 0)
		for _, service := range existingServices {
			if service.GitRepoPath != "" && service.GitRef != "" {
				services = append(services, service)
			}
		}
		return services, nil
	}

	serviceRefs := core.ExtractServiceReferences(script)

	services := make([]domain.Service, 0)
	for _, ref := range serviceRefs {
		service, err := findService(ref, existingServices)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}

func findService(serviceName string, existingServices []domain.Service) (domain.Service, error) {
	for _, service := range existingServices {
		if service.Name == serviceName {
			return service, nil
		}
	}
	return domain.Service{}, fmt.Errorf("service '%s' not found", serviceName)
}

func getShellCommand() (shell string, shellArg string) {
	return "bash", "-c"
}
