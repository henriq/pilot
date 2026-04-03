package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"dx/cmd/cli/app"
	"dx/internal/cli/output"
	"dx/internal/core/domain"

	"github.com/spf13/cobra"
)

func init() {
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextInfoCmd)
	contextCmd.AddCommand(contextPrintCmd)
	contextCmd.AddCommand(contextSetCmd)
	rootCmd.AddCommand(contextCmd)
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manages the configuration context",
	Long:  `Commands for managing and viewing the configuration context`,
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists the available contexts",
	Long:  `Reads the available contexts from the configuration file and prints them to stdout`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectContextCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleList()
	},
}

var contextPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "Prints the current context",
	Long:  `Prints the current context as json to stdout`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectContextCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandlePrint()
	},
}

var contextInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Prints info about the current context",
	Long:  `Prints the available services and their profiles in the current context`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configRepo, err := app.InjectConfigRepo()
		if err != nil {
			return fmt.Errorf("error injecting config repo: %v", err)
		}
		configContext, err := configRepo.LoadCurrentConfigurationContext()
		if err != nil {
			return fmt.Errorf("error loading current configuration context: %v", err)
		}

		fmt.Println("Current context: " + output.Bold(configContext.Name))
		fmt.Println()
		fmt.Printf("%s\n", output.Header(fmt.Sprintf("%-30s %-30s", "Service", "Profiles")))

		slices.SortFunc(
			configContext.Services, func(a, b domain.Service) int {
				return strings.Compare(a.Name, b.Name)
			},
		)

		for _, service := range configContext.Services {
			fmt.Printf("%-30s %-30s\n", service.Name, getSortedProfiles(service.Profiles))
		}
		fmt.Println()

		fmt.Printf(
			"%s\n",
			output.Header(fmt.Sprintf("%-30s %-12s %-16s %-30s %-30s %-50s",
				"Local service",
				"Local port",
				"Kubernetes port",
				"Health check",
				"Selector",
				"Ingress",
			)),
		)

		slices.SortFunc(
			configContext.LocalServices, func(a, b domain.LocalService) int {
				return strings.Compare(a.Name, b.Name)
			},
		)

		for _, service := range configContext.LocalServices {
			fmt.Printf(
				"%-30s %-12s %-16s %-30s %-30s %-50s\n",
				service.Name,
				formatPort(service.LocalPort),
				formatPort(service.KubernetesPort),
				service.HealthCheckPath,
				formatSelector(service.Selector),
				formatIngress(service, *configContext),
			)
		}
		fmt.Println()
		fmt.Println("mitmweb: " + output.Bold(fmt.Sprintf("http://dev-proxy.%s.localhost", configContext.Name)))
		fmt.Println("haproxy stats: " + output.Bold(fmt.Sprintf("http://stats.dev-proxy.%s.localhost", configContext.Name)))
		return nil
	},
}

func formatIngress(service domain.LocalService, ctx domain.ConfigurationContext) string {
	return fmt.Sprintf("http://%s.%s.localhost", service.Name, ctx.Name)
}

func formatSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "-"
	}
	var formatted []string
	for key, value := range selector {
		formatted = append(formatted, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(formatted, ", ")
}

func formatPort(port int) string {
	if port == 0 {
		return "-"
	}
	return strconv.Itoa(port)
}

func getSortedProfiles(profiles []string) string {
	if len(profiles) == 0 {
		return ""
	}
	slices.Sort(profiles)
	return strings.Join(profiles, ", ")
}

var contextSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Sets the current context",
	Long:  `Sets the current context to the specified context`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		configRepo, err := app.InjectConfigRepo()
		if err != nil {
			return fmt.Errorf("error injecting config repo: %v", err)
		}
		config, err := configRepo.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}

		for _, context := range config.Contexts {
			if context.Name == args[0] {
				return nil
			}
		}
		return fmt.Errorf("context '%s' not found", args[0])
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		configRepo, err := app.InjectConfigRepo()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		config, err := configRepo.LoadConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var contexts []string
		for _, context := range config.Contexts {
			contexts = append(contexts, context.Name)
		}
		return contexts, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := app.InjectContextCommandHandler()
		if err != nil {
			return err
		}

		return handler.HandleSet(args[0])
	},
}
