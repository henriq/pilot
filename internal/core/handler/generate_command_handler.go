package handler

import (
	"dx/internal/core/domain"
	"dx/internal/ports"
	"fmt"
	"io"
	"slices"
	"strings"
)

type GenerateCommandHandler struct {
	configRepository ports.ConfigRepository
}

func ProvideGenerateCommandHandler(
	configRepository ports.ConfigRepository,
) GenerateCommandHandler {
	return GenerateCommandHandler{
		configRepository: configRepository,
	}
}

func (h *GenerateCommandHandler) HandleGenerateHostEntries(out io.Writer) error {
	context, err := h.configRepository.LoadCurrentConfigurationContext()

	if err != nil {
		return err
	}

	fmt.Fprintf(out, "# DX entries for %s\n", context.Name)
	fmt.Fprintf(out, "127.0.0.1 dev-proxy.%s.localhost\n", context.Name)
	fmt.Fprintf(out, "127.0.0.1 stats.dev-proxy.%s.localhost\n", context.Name)

	if len(context.LocalServices) > 0 {
		localServices := context.LocalServices
		slices.SortFunc(
			localServices, func(a, b domain.LocalService) int {
				return strings.Compare(a.Name, b.Name)
			},
		)
		for _, localService := range localServices {
			fmt.Fprintf(out, "127.0.0.1 %s.%s.localhost\n", localService.Name, context.Name)
		}
	}

	return nil
}
