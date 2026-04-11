package cmd

import (
	"fmt"

	"pilot/cmd/cli/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:               "run [script...]",
	Short:             "Run a custom script defined in the configuration",
	Long:              `Run one or more scripts defined in the current context's configuration.`,
	Args:              ScriptArgsValidator,
	ValidArgsFunction: ScriptArgsCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectRunCommandHandler()
		if err != nil {
			return err
		}
		configRepo, err := app.InjectConfigRepo()
		if err != nil {
			return fmt.Errorf("error injecting config repo: %v", err)
		}
		configContext, err := configRepo.LoadCurrentConfigurationContext()
		if err != nil {
			return fmt.Errorf("error loading current configuration context: %v", err)
		}

		scripts := make(map[string]string)
		executionPlan := make([]string, 0)
		for _, wantedScriptName := range args {
			for scriptName, script := range configContext.Scripts {
				if wantedScriptName == scriptName {
					scripts[scriptName] = script
					executionPlan = append(executionPlan, scriptName)
				}
			}
		}

		return handler.Handle(scripts, executionPlan)
	},
}

func ScriptArgsValidator(cmd *cobra.Command, args []string) error {
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return fmt.Errorf("error injecting config repo: %v", err)
	}
	configContext, err := configRepo.LoadCurrentConfigurationContext()
	if err != nil {
		return fmt.Errorf("error loading current configuration context: %v", err)
	}

	for _, wantedScriptName := range args {
		var foundScript bool
		for scriptName := range configContext.Scripts {
			if wantedScriptName == scriptName {
				foundScript = true
				break
			}
		}
		if !foundScript {
			return fmt.Errorf("script %s not found", wantedScriptName)
		}
	}

	return nil
}

func ScriptArgsCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	configRepo, err := app.InjectConfigRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	configContext, err := configRepo.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var matchingScripts []string
	for scriptName := range configContext.Scripts {
		matchingScripts = append(matchingScripts, scriptName)
	}

	return matchingScripts, cobra.ShellCompDirectiveNoFileComp
}
