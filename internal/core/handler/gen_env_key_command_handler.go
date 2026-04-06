package handler

import (
	"fmt"
	"path/filepath"

	"dx/internal/cli/output"
	"dx/internal/ports"
)

type GenEnvKeyCommandHandler struct {
	configRepository      ports.ConfigRepository
	fileSystem            ports.FileSystem
	containerOrchestrator ports.ContainerOrchestrator
}

func ProvideGenEnvKeyCommandHandler(
	configRepository ports.ConfigRepository,
	fileSystem ports.FileSystem,
	containerOrchestrator ports.ContainerOrchestrator,
) GenEnvKeyCommandHandler {
	return GenEnvKeyCommandHandler{
		configRepository:      configRepository,
		fileSystem:            fileSystem,
		containerOrchestrator: containerOrchestrator,
	}
}

func (h *GenEnvKeyCommandHandler) Handle() error {
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	envKey, err := h.containerOrchestrator.CreateClusterEnvironmentKey()
	if err != nil {
		return fmt.Errorf("failed to generate environment key: %v", err)
	}
	envKeyPath := filepath.Join("~", ".dx", configContext.Name, "env-key")
	err = h.fileSystem.WriteFile(envKeyPath, []byte(envKey), ports.ReadWrite)
	if err != nil {
		return err
	}
	output.PrintSuccess("Environment key generated")
	return nil
}
