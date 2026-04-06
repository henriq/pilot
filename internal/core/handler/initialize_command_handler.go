package handler

import (
	"fmt"

	"dx/internal/cli/output"
	"dx/internal/core/domain"
	"dx/internal/ports"
)

type InitializeCommandHandler struct {
	configRepository ports.ConfigRepository
}

func ProvideInitializeCommandHandler(
	configRepository ports.ConfigRepository,
) InitializeCommandHandler {
	return InitializeCommandHandler{
		configRepository: configRepository,
	}
}

func (h *InitializeCommandHandler) Handle() error {
	configExists, err := h.configRepository.ConfigExists()
	if err != nil {
		return err
	}
	if configExists {
		return fmt.Errorf("configuration already exists at ~/.dx-config.yaml")
	}
	config := domain.CreateDefaultConfig()
	err = h.configRepository.SaveConfig(&config)
	if err != nil {
		return err
	}

	output.PrintSuccess("Configuration created at ~/.dx-config.yaml")
	return nil
}
