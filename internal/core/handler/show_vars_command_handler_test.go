package handler

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowVarsCommandHandler_Handle_Success(t *testing.T) {
	secretsRepository := new(testutil.MockSecretsRepository)
	configRepository := new(testutil.MockConfigRepository)

	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		Services: []domain.Service{
			{Name: "service-1", Path: "/path/to/service"},
		},
	}
	secrets := []*domain.Secret{
		{Key: "DB_PASSWORD", Value: "secret123"},
	}

	// For CreateTemplatingValues
	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	sut := NewShowVarsCommandHandler(secretsRepository, configRepository)

	err := sut.Handle()

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestShowVarsCommandHandler_Handle_NoSecrets(t *testing.T) {
	secretsRepository := new(testutil.MockSecretsRepository)
	configRepository := new(testutil.MockConfigRepository)

	configContext := &domain.ConfigurationContext{
		Name:     "test-context",
		Services: []domain.Service{},
	}
	secrets := []*domain.Secret{}

	// For CreateTemplatingValues
	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	sut := NewShowVarsCommandHandler(secretsRepository, configRepository)

	err := sut.Handle()

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestShowVarsCommandHandler_Handle_LoadContextNameError(t *testing.T) {
	secretsRepository := new(testutil.MockSecretsRepository)
	configRepository := new(testutil.MockConfigRepository)

	expectedErr := errors.New("load context name error")
	configRepository.On("LoadCurrentContextName").Return("", expectedErr)

	sut := NewShowVarsCommandHandler(secretsRepository, configRepository)

	err := sut.Handle()

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
}

func TestShowVarsCommandHandler_Handle_LoadConfigError(t *testing.T) {
	secretsRepository := new(testutil.MockSecretsRepository)
	configRepository := new(testutil.MockConfigRepository)

	expectedErr := errors.New("load config error")
	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, expectedErr)

	sut := NewShowVarsCommandHandler(secretsRepository, configRepository)

	err := sut.Handle()

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
}

func TestShowVarsCommandHandler_Handle_LoadSecretsError(t *testing.T) {
	secretsRepository := new(testutil.MockSecretsRepository)
	configRepository := new(testutil.MockConfigRepository)

	configContext := &domain.ConfigurationContext{Name: "test-context"}
	expectedErr := errors.New("load secrets error")

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(nil, expectedErr)

	sut := NewShowVarsCommandHandler(secretsRepository, configRepository)

	err := sut.Handle()

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestPrettyPrintMap_NonStringNonMapValues(t *testing.T) {
	// Verify that non-string, non-map values are printed without panicking
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	prettyPrintMap(map[string]interface{}{
		"count":   42,
		"enabled": true,
		"items":   []string{"a", "b"},
		"nothing": nil,
	}, 0, false)

	w.Close() //nolint:errcheck,gosec // test pipe close
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	output := buf.String()

	assert.Contains(t, output, "count: 42")
	assert.Contains(t, output, "enabled: true")
	assert.Contains(t, output, "items: [a b]")
	assert.Contains(t, output, "nothing: <nil>")
}

func TestPrettyPrintMap_SecretsKeyHidesNestedValues(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	prettyPrintMap(map[string]interface{}{
		"Secrets": map[string]interface{}{
			"DB_PASSWORD": "supersecret",
		},
	}, 0, false)

	w.Close() //nolint:errcheck,gosec // test pipe close
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	output := buf.String()

	assert.Contains(t, output, "DB_PASSWORD: ******")
	assert.NotContains(t, output, "supersecret")
}

func TestShowVarsCommandHandler_Handle_SortedOutput(t *testing.T) {
	secretsRepository := new(testutil.MockSecretsRepository)
	configRepository := new(testutil.MockConfigRepository)

	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		Services: []domain.Service{
			{Name: "zebra-service", Path: "/path/to/zebra"},
			{Name: "alpha-service", Path: "/path/to/alpha"},
			{Name: "mike-service", Path: "/path/to/mike"},
		},
	}
	secrets := []*domain.Secret{
		{Key: "ZEBRA_SECRET", Value: "zebra-value"},
		{Key: "ALPHA_SECRET", Value: "alpha-value"},
		{Key: "MIKE_SECRET", Value: "mike-value"},
	}

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	sut := NewShowVarsCommandHandler(secretsRepository, configRepository)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := sut.Handle()

	w.Close() //nolint:errcheck,gosec // test pipe close
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	output := buf.String()

	require.NoError(t, err)

	// Verify secrets are sorted alphabetically
	alphaSecretPos := bytes.Index([]byte(output), []byte("ALPHA_SECRET"))
	mikeSecretPos := bytes.Index([]byte(output), []byte("MIKE_SECRET"))
	zebraSecretPos := bytes.Index([]byte(output), []byte("ZEBRA_SECRET"))

	assert.True(t, alphaSecretPos < mikeSecretPos, "ALPHA_SECRET should appear before MIKE_SECRET")
	assert.True(t, mikeSecretPos < zebraSecretPos, "MIKE_SECRET should appear before ZEBRA_SECRET")

	// Verify services are sorted alphabetically
	alphaServicePos := bytes.Index([]byte(output), []byte("alpha-service"))
	mikeServicePos := bytes.Index([]byte(output), []byte("mike-service"))
	zebraServicePos := bytes.Index([]byte(output), []byte("zebra-service"))

	assert.True(t, alphaServicePos < mikeServicePos, "alpha-service should appear before mike-service")
	assert.True(t, mikeServicePos < zebraServicePos, "mike-service should appear before zebra-service")

	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}
