package handler

import (
	"pilot/internal/core/domain"
	"pilot/internal/testutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCommandHandler_GenerateHostEntriesReturnsHostEntries(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		LocalServices: []domain.LocalService{
			{
				Name:      "test-service",
				LocalPort: 8080,
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	sut := GenerateCommandHandler{
		configRepository: configRepository,
	}

	output := strings.Builder{}
	result := sut.HandleGenerateHostEntries(&output)

	assert.Nil(t, result)
	assert.Equal(
		t, `# Pilot entries for test-context
127.0.0.1 dev-proxy.test-context.localhost
127.0.0.1 stats.dev-proxy.test-context.localhost
127.0.0.1 test-service.test-context.localhost
`, output.String(),
	)
}
