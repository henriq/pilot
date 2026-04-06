package handler

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"dx/internal/cli/output"
	"dx/internal/cli/progress"
	"dx/internal/ports"

	"golang.org/x/sync/errgroup"
)

const maxConcurrentPulls = 5

type PullCommandHandler struct {
	configRepository         ports.ConfigRepository
	containerImageRepository ports.ContainerImageRepository
	terminalInput            ports.TerminalInput
}

func ProvidePullCommandHandler(
	configRepository ports.ConfigRepository,
	containerImageRepository ports.ContainerImageRepository,
	terminalInput ports.TerminalInput,
) PullCommandHandler {
	return PullCommandHandler{
		configRepository:         configRepository,
		containerImageRepository: containerImageRepository,
		terminalInput:            terminalInput,
	}
}

func (h *PullCommandHandler) Handle(services []string, selectedProfile string, skipConfirmation bool) error {
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	var remoteImages []string
	var dockerImageNames []string

	for _, service := range configContext.FilterServices(services, selectedProfile) {
		remoteImages = append(remoteImages, service.RemoteImages...)

		for _, dockerImage := range service.DockerImages {
			dockerImageNames = append(dockerImageNames, dockerImage.Name)
		}
	}

	// Combine and deduplicate all images
	allImages := make([]string, 0, len(remoteImages)+len(dockerImageNames))
	allImages = append(allImages, remoteImages...)
	allImages = append(allImages, dockerImageNames...)
	allImages = deduplicate(allImages)
	slices.Sort(allImages)

	// Deduplicate docker image names for the confirmation message
	dockerImageNames = deduplicate(dockerImageNames)
	slices.Sort(dockerImageNames)

	if len(allImages) == 0 {
		output.PrintInfo("No images to pull")
		return nil
	}

	// Confirm if docker images will be overwritten
	if len(dockerImageNames) > 0 && !skipConfirmation {
		if !h.terminalInput.IsTerminal() {
			return fmt.Errorf("pulling locally-built images requires confirmation. Use --yes to skip in non-interactive mode")
		}

		output.PrintWarning(
			fmt.Sprintf(
				"Pulling these images will overwrite %d locally-built %s:",
				len(dockerImageNames),
				output.Plural(len(dockerImageNames), "image", "images"),
			),
		)
		fmt.Println()
		for _, name := range dockerImageNames {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()

		response, err := h.terminalInput.ReadLine("Continue? [y/N] ")
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			output.PrintInfo("Pull cancelled")
			return nil
		}
		fmt.Println()
	}

	pullStartTime := time.Now()
	output.PrintHeader("Pulling Docker images")
	fmt.Println()

	tracker := progress.NewConcurrentTracker(allImages, "Pulling")
	tracker.Start()

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(maxConcurrentPulls)

	for i, image := range allImages {
		g.Go(
			func() error {
				// Check if context is canceled
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				tracker.StartItem(i)

				err := h.containerImageRepository.PullImage(image)
				tracker.CompleteItem(i, err)

				return err
			},
		)
	}

	pullErr := g.Wait()
	tracker.Stop()

	if pullErr != nil {
		return pullErr
	}

	fmt.Println()
	output.PrintSuccess(
		fmt.Sprintf(
			"Pulled %d Docker %s in %s",
			len(allImages),
			output.Plural(len(allImages), "image", "images"),
			progress.FormatDuration(time.Since(pullStartTime)),
		),
	)

	return nil
}

func deduplicate(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
