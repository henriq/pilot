package core

import (
	"fmt"

	"dx/internal/ports"
)

type EnvironmentEnsurer struct {
	configRepository      ports.ConfigRepository
	containerOrchestrator ports.ContainerOrchestrator
}

func ProvideEnvironmentEnsurer(
	configRepository ports.ConfigRepository,
	kubernetesService ports.ContainerOrchestrator,
) EnvironmentEnsurer {
	return EnvironmentEnsurer{
		configRepository:      configRepository,
		containerOrchestrator: kubernetesService,
	}
}

func (ee *EnvironmentEnsurer) EnsureExpectedClusterIsSelected() error {
	currentKey, err := ee.containerOrchestrator.CreateClusterEnvironmentKey()
	if err != nil {
		return fmt.Errorf("failed to generate current environment key: %v", err)
	}

	configContext, err := ee.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return fmt.Errorf("error loading current configuration context: %v", err)
	}
	envKey, err := ee.configRepository.LoadEnvKey(configContext.Name)
	if err != nil {
		return err
	}

	if envKey != currentKey {
		return fmt.Errorf("environment key mismatch, please verify that the correct cluster and namespace are active or run 'dx gen-env-key' to update the env-key")
	}

	return nil
}
