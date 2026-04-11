package handler

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"pilot/internal/cli/output"
	"pilot/internal/cli/progress"
	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

type BuildCommandHandler struct {
	configRepository         ports.ConfigRepository
	scm                      ports.Scm
	containerImageRepository ports.ContainerImageRepository
}

func NewBuildCommandHandler(
	configRepository ports.ConfigRepository,
	scm ports.Scm,
	containerImageRepository ports.ContainerImageRepository,
) BuildCommandHandler {
	return BuildCommandHandler{
		configRepository:         configRepository,
		scm:                      scm,
		containerImageRepository: containerImageRepository,
	}
}

func (h *BuildCommandHandler) Handle(services []string, selectedProfile string) error {
	var dockerImagesToBuild []domain.DockerImage
	var dockerImagesToPull []string

	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	for _, service := range configContext.FilterServices(services, selectedProfile) {
		dockerImagesToBuild = append(dockerImagesToBuild, service.DockerImages...)
		dockerImagesToPull = append(dockerImagesToPull, service.RemoteImages...)
	}

	if len(dockerImagesToBuild) > 0 {
		slices.SortFunc(
			dockerImagesToBuild, func(a, b domain.DockerImage) int {
				return strings.Compare(a.Name, b.Name)
			},
		)

		buildStartTime := time.Now()
		output.PrintHeader("Building Docker images")
		fmt.Println()

		// Create progress tracker with repo/ref info
		imageNames := make([]string, len(dockerImagesToBuild))
		imageInfos := make([]string, len(dockerImagesToBuild))
		for i, img := range dockerImagesToBuild {
			imageNames[i] = img.Name
			if img.GitRepoPath != "" && img.GitRef != "" {
				imageInfos[i] = fmt.Sprintf("%s @ %s", img.GitRepoPath, img.GitRef)
			}
		}
		tracker := progress.NewTrackerWithInfoAndVerb(imageNames, imageInfos, "Building")
		tracker.Start()

		var buildErr error
		for i, image := range dockerImagesToBuild {
			tracker.StartItem(i)

			// Show dockerfile override note (only in non-TTY mode, TTY shows spinner)
			if image.DockerfileOverride != "" {
				output.PrintSecondary("Using inline Dockerfile from configuration")
			}

			// Download source
			if err := h.scm.Download(image.GitRepoPath, image.GitRef, image.Path); err != nil {
				tracker.CompleteItem(i, err)
				tracker.PrintItemComplete(i)
				buildErr = err
				break
			}

			// Build image
			if err := h.containerImageRepository.BuildImage(image); err != nil {
				tracker.CompleteItem(i, err)
				tracker.PrintItemComplete(i)
				buildErr = err
				break
			}

			tracker.CompleteItem(i, nil)
			tracker.PrintItemComplete(i)
		}

		tracker.Stop()

		if buildErr != nil {
			return buildErr
		}

		fmt.Println()
		output.PrintSuccess(fmt.Sprintf("Built %d Docker %s in %s", len(dockerImagesToBuild), output.Plural(len(dockerImagesToBuild), "image", "images"), progress.FormatDuration(time.Since(buildStartTime))))
		fmt.Println()
	}

	if len(dockerImagesToPull) > 0 {
		slices.Sort(dockerImagesToPull)

		pullStartTime := time.Now()
		output.PrintHeader("Pulling Docker images")
		fmt.Println()

		// Create progress tracker for pulls
		tracker := progress.NewTrackerWithVerb(dockerImagesToPull, "Pulling")
		tracker.Start()

		var pullErr error
		for i, image := range dockerImagesToPull {
			tracker.StartItem(i)

			if err := h.containerImageRepository.PullImage(image); err != nil {
				tracker.CompleteItem(i, err)
				tracker.PrintItemComplete(i)
				pullErr = err
				break
			}

			tracker.CompleteItem(i, nil)
			tracker.PrintItemComplete(i)
		}

		tracker.Stop()

		if pullErr != nil {
			return pullErr
		}

		fmt.Println()
		output.PrintSuccess(fmt.Sprintf("Pulled %d Docker %s in %s", len(dockerImagesToPull), output.Plural(len(dockerImagesToPull), "image", "images"), progress.FormatDuration(time.Since(pullStartTime))))
	}

	return nil
}
