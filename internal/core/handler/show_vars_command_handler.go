package handler

import (
	"fmt"
	"sort"
	"strings"

	"dx/internal/core"
	"dx/internal/ports"
)

type ShowVarsCommandHandler struct {
	secretsRepository ports.SecretsRepository
	configRepository  ports.ConfigRepository
}

func ProvideShowVarsCommandHandler(
	secretsRepository ports.SecretsRepository,
	configRepository ports.ConfigRepository,
) ShowVarsCommandHandler {
	return ShowVarsCommandHandler{
		secretsRepository: secretsRepository,
		configRepository:  configRepository,
	}
}

func (h *ShowVarsCommandHandler) Handle() error {
	values, err := core.CreateTemplatingValues(h.configRepository, h.secretsRepository)
	if err != nil {
		return err
	}

	prettyPrintValuesMap(values)

	return nil
}

func prettyPrintValuesMap(values map[string]interface{}) {
	prettyPrintMap(values, 0, false)
}

func prettyPrintMap(values map[string]interface{}, indent int, hidden bool) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	indentString := strings.Repeat(" ", indent)
	for _, key := range keys {
		value := values[key]
		if _, ok := value.(string); ok {
			if hidden {
				fmt.Printf("%s%s: ******\n", indentString, key)
			} else {
				fmt.Printf("%s%s: %s\n", indentString, key, value)
			}
		} else if nested, ok := value.(map[string]interface{}); ok {
			fmt.Printf("%s%s:\n", indentString, key)
			if strings.Contains(key, "Secrets") {
				prettyPrintMap(nested, indent+2, true)
			} else {
				prettyPrintMap(nested, indent+2, hidden)
			}
		} else {
			fmt.Printf("%s%s: %v\n", indentString, key, value)
		}
	}
}
