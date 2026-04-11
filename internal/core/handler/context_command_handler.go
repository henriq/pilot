package handler

import (
	"encoding/json"
	"fmt"

	"pilot/internal/cli/output"
	"pilot/internal/ports"
)

type ContextCommandHandler struct {
	configRepository ports.ConfigRepository
}

func NewContextCommandHandler(
	configRepository ports.ConfigRepository,
) ContextCommandHandler {
	return ContextCommandHandler{
		configRepository: configRepository,
	}
}

func (h *ContextCommandHandler) HandleSet(contextName string) error {
	config, err := h.configRepository.LoadConfig()
	if err != nil {
		return err
	}
	if !config.ContextExists(contextName) {
		return fmt.Errorf("context not found: %s", contextName)
	}
	err = h.configRepository.SaveCurrentContextName(contextName)
	if err != nil {
		return err
	}
	output.PrintSuccess(fmt.Sprintf("Switched to context '%s'", contextName))
	return nil
}

func (h *ContextCommandHandler) HandleList() error {
	config, err := h.configRepository.LoadConfig()
	if err != nil {
		return err
	}

	currentContext, _ := h.configRepository.LoadCurrentContextName()

	if len(config.Contexts) == 0 {
		output.PrintInfo("No contexts configured")
		return nil
	}

	output.PrintHeader("Contexts")
	fmt.Println()

	for _, context := range config.Contexts {
		if context.Name == currentContext {
			fmt.Printf("  %s %s %s\n", output.SymbolArrow, output.Bold(context.Name), output.Dim("(current)"))
		} else {
			fmt.Printf("  %s %s\n", output.SymbolBullet, context.Name)
		}
	}
	return nil
}

func (h *ContextCommandHandler) HandlePrint() error {
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	return prettyPrint(configContext)
}

func prettyPrint(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
