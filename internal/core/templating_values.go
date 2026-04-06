package core

import (
	"strings"

	"dx/internal/core/domain"
	"dx/internal/ports"
)

func CreateTemplatingValues(
	configRepository ports.ConfigRepository,
	secretsRepository ports.SecretsRepository,
) (map[string]interface{}, error) {
	contextName, err := configRepository.LoadCurrentContextName()
	if err != nil {
		return nil, err
	}
	configContext, err := configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, err
	}
	secrets, err := secretsRepository.LoadSecrets(contextName)
	if err != nil {
		return nil, err
	}

	secretsMap := createSecretsMap(secrets)
	servicesMap := createServicesMap(configContext)
	values := map[string]interface{}{
		"Secrets":  secretsMap,
		"Services": servicesMap,
	}
	return values, nil
}

func createServicesMap(configContext *domain.ConfigurationContext) map[string]interface{} {
	servicesMap := make(map[string]interface{})
	for _, service := range configContext.Services {
		serviceHasValue := false
		serviceMap := make(map[string]interface{})
		if service.Path != "" {
			// Convert backslashes to forward slashes so Windows paths are not
			// misinterpreted as escape sequences when used in bash scripts.
			serviceMap["path"] = strings.ReplaceAll(service.Path, "\\", "/")
			serviceHasValue = true
		}
		if service.GitRef != "" {
			serviceMap["gitRef"] = service.GitRef
			serviceHasValue = true
		}

		if serviceHasValue {
			servicesMap[service.Name] = serviceMap
		}
	}

	return servicesMap
}

// Create secrets map, splitting strings by "." to create nested maps
func createSecretsMap(secrets []*domain.Secret) map[string]interface{} {
	secretMap := make(map[string]interface{})
	for _, secret := range secrets {
		parts := strings.Split(secret.Key, ".")
		currentMap := secretMap
		for i, part := range parts {
			if i == len(parts)-1 {
				currentMap[part] = secret.Value
			} else {
				if currentMap[part] == nil {
					currentMap[part] = make(map[string]interface{})
				}
				if nested, ok := currentMap[part].(map[string]interface{}); ok {
					currentMap = nested
				} else {
					// Key conflict: a value already exists at this path but isn't a map.
					// Skip this secret to avoid overwriting the existing value.
					break
				}
			}
		}
	}
	return secretMap
}
