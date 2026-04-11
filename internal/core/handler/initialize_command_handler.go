package handler

import (
	"fmt"

	"pilot/internal/cli/output"
	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

type InitializeCommandHandler struct {
	configRepository ports.ConfigRepository
}

func NewInitializeCommandHandler(
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
		return fmt.Errorf("configuration already exists at ~/.pilot-config.yaml")
	}
	config := domain.CreateDefaultConfig()
	err = h.configRepository.SaveConfig(&config)
	if err != nil {
		return err
	}

	output.PrintSuccess("Configuration created at ~/.pilot-config.yaml")
	return nil
}
