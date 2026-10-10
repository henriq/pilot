package arguments

import (
	"strings"

	"pilot/cmd/cli/app"
	"pilot/internal/core/domain"

	"github.com/spf13/cobra"
)

// RegisterHelmChartOverrideFlag wires the --helm-chart flag onto cmd, binding
// repeated "service:path" values into target.
func RegisterHelmChartOverrideFlag(cmd *cobra.Command, target *[]string) {
	cmd.Flags().StringArrayVar(
		target,
		"helm-chart",
		nil,
		"render a service's Helm chart from a local directory instead of cloning the repo (useful for chart development); format: service:path; repeat for multiple services",
	)
	_ = cmd.RegisterFlagCompletionFunc("helm-chart", HelmChartOverrideCompletion)
}

// HelmChartOverrideCompletion offers service names with a trailing ":" until
// the user types one, then switches to directory completion for the path.
func HelmChartOverrideCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if separatorIndex := strings.Index(toComplete, ":"); separatorIndex >= 0 {
		servicePrefix := toComplete[:separatorIndex+1]
		pathToComplete := toComplete[separatorIndex+1:]
		return prefixedDirectoryCompletions(servicePrefix, pathToComplete), cobra.ShellCompDirectiveNoSpace
	}
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	configContext, err := configRepo.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return serviceNameColonCompletions(configContext.Services), cobra.ShellCompDirectiveNoSpace
}

// ParseHelmChartOverrides converts repeated --helm-chart entries in
// "service:path" form into a HelmChartOverrides value.
func ParseHelmChartOverrides(entries []string) (domain.HelmChartOverrides, error) {
	chartDirectoryByService, err := nameValueFlag{
		flagName:       "helm-chart",
		formatHint:     "service:path",
		nameNoun:       "service",
		valueNoun:      "chart path",
		normalizeValue: resolveExistingDirectory,
	}.parse(entries)
	if err != nil {
		return domain.HelmChartOverrides{}, err
	}
	return domain.NewHelmChartOverrides(chartDirectoryByService), nil
}
