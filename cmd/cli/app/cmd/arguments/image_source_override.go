package arguments

import (
	"strings"

	"pilot/cmd/cli/app"
	"pilot/internal/core/domain"

	"github.com/spf13/cobra"
)

// RegisterImageSourceOverrideFlag wires the --image-source flag onto cmd,
// binding repeated "image:path" values into target.
func RegisterImageSourceOverrideFlag(cmd *cobra.Command, target *[]string) {
	cmd.Flags().StringArrayVar(
		target,
		"image-source",
		nil,
		"build a Docker image from a local directory instead of cloning the repo (useful for testing local image changes); format: image:path; repeat for multiple images",
	)
	_ = cmd.RegisterFlagCompletionFunc("image-source", ImageSourceOverrideCompletion)
}

// ImageSourceOverrideCompletion offers Docker image names with a trailing ":"
// until the user types one, then switches to directory completion for the path.
func ImageSourceOverrideCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if separatorIndex := strings.Index(toComplete, ":"); separatorIndex >= 0 {
		imagePrefix := toComplete[:separatorIndex+1]
		pathToComplete := toComplete[separatorIndex+1:]
		return prefixedDirectoryCompletions(imagePrefix, pathToComplete), cobra.ShellCompDirectiveNoSpace
	}
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	configContext, err := configRepo.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return imageSourceOverrideImageCompletions(configContext.Services), cobra.ShellCompDirectiveNoSpace
}

func imageSourceOverrideImageCompletions(services []domain.Service) []cobra.Completion {
	seen := map[string]struct{}{}
	var completions []cobra.Completion
	for _, service := range services {
		for _, image := range service.DockerImages {
			if _, ok := seen[image.Name]; ok {
				continue
			}
			seen[image.Name] = struct{}{}
			completions = append(completions, image.Name+":")
		}
	}
	return completions
}

// ParseImageSourceOverrides converts repeated --image-source entries in
// "image:path" form into a DockerImageSourceOverrides value.
func ParseImageSourceOverrides(entries []string) (domain.DockerImageSourceOverrides, error) {
	sourcePathByImage, err := nameValueFlag{
		flagName:       "image-source",
		formatHint:     "image:path",
		nameNoun:       "image",
		valueNoun:      "source path",
		normalizeValue: resolveExistingDirectory,
	}.parse(entries)
	if err != nil {
		return domain.DockerImageSourceOverrides{}, err
	}
	return domain.NewDockerImageSourceOverrides(sourcePathByImage), nil
}
