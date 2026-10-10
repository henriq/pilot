package arguments

import (
	"strings"

	"pilot/cmd/cli/app"
	"pilot/internal/core/domain"

	"github.com/spf13/cobra"
)

// RegisterServiceGitRefOverrideFlag wires the --git-ref flag onto cmd,
// binding repeated "service:ref" values into target.
func RegisterServiceGitRefOverrideFlag(cmd *cobra.Command, target *[]string) {
	cmd.Flags().StringArrayVar(
		target,
		"git-ref",
		nil,
		"build a service's Docker image from a different git ref than the configured one (useful for testing feature branches); format: service:ref; repeat for multiple services",
	)
	_ = cmd.RegisterFlagCompletionFunc("git-ref", GitRefOverrideCompletion)
}

// GitRefOverrideCompletion offers service names with a trailing ":" until the
// user types one, then yields no completion since refs are free-form strings.
func GitRefOverrideCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ":") {
		return nil, cobra.ShellCompDirectiveNoFileComp
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

// ParseServiceGitRefOverrides converts repeated --git-ref entries in
// "service:ref" form into a ServiceGitRefOverrides value. Refs are shape-checked
// so a hostile value cannot be misread by git as a command-line option.
func ParseServiceGitRefOverrides(entries []string) (domain.ServiceGitRefOverrides, error) {
	gitRefByService, err := nameValueFlag{
		flagName:   "git-ref",
		formatHint: "service:ref",
		nameNoun:   "service",
		valueNoun:  "git ref",
		normalizeValue: func(gitRef string) (string, error) {
			return gitRef, domain.ValidateGitRefShape(gitRef)
		},
	}.parse(entries)
	if err != nil {
		return domain.ServiceGitRefOverrides{}, err
	}
	return domain.NewServiceGitRefOverrides(gitRefByService)
}
