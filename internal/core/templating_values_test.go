package core

import (
	"testing"

	"dx/internal/core/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSecretsMap(t *testing.T) {
	secrets := []*domain.Secret{
		{Key: "simple", Value: "value1"},
		{Key: "nested.key", Value: "value2"},
		{Key: "deeply.nested.key", Value: "value3"},
	}

	result := createSecretsMap(secrets)

	assert.Equal(t, "value1", result["simple"])
	assert.Equal(t, "value2", result["nested"].(map[string]interface{})["key"])
	assert.Equal(t, "value3", result["deeply"].(map[string]interface{})["nested"].(map[string]interface{})["key"])
}

func TestCreateSecretsMap_ConflictingKeys(t *testing.T) {
	// Legacy data may contain conflicting keys (e.g., "db" as scalar and "db.password" as nested).
	// createSecretsMap skips the nested key when a scalar already exists at the path prefix.
	secrets := []*domain.Secret{
		{Key: "db", Value: "connection-string"},
		{Key: "db.password", Value: "secret123"},
	}

	result := createSecretsMap(secrets)

	// The first key should be preserved
	assert.Equal(t, "connection-string", result["db"])

	// The conflicting nested key should be skipped (db is not a map, so db.password can't be set)
	// Verify db is still a string, not a map
	_, isMap := result["db"].(map[string]interface{})
	assert.False(t, isMap, "db should remain a string, not be converted to a map")
}

func TestCreateSecretsMap_ConflictingKeys_ReverseOrder(t *testing.T) {
	// Reverse conflict: nested key stored before scalar at the same path.
	// The scalar overwrites the map (last-wins for terminal values).
	secrets := []*domain.Secret{
		{Key: "db.password", Value: "secret123"}, // Creates db as a map
		{Key: "db", Value: "connection-string"},  // Overwrites db map with scalar
	}

	result := createSecretsMap(secrets)

	// The last scalar value should win (last-wins behavior for terminal values)
	assert.Equal(t, "connection-string", result["db"])

	// db is now a scalar, not a map
	_, isMap := result["db"].(map[string]interface{})
	assert.False(t, isMap, "db should be a string after scalar overwrites map")
}

func TestCreateServicesMap(t *testing.T) {
	context := &domain.ConfigurationContext{
		Services: []domain.Service{
			{Name: "svc1", Path: "/path/to/svc1", GitRef: "main"},
			{Name: "svc2", Path: "", GitRef: ""}, // No path or gitRef
		},
	}

	result := createServicesMap(context)

	// svc1 should have entries
	svc1, ok := result["svc1"].(map[string]interface{})
	require.True(t, ok, "svc1 should be present")
	assert.Equal(t, "/path/to/svc1", svc1["path"])
	assert.Equal(t, "main", svc1["gitRef"])

	// svc2 should not be present (no values)
	_, ok = result["svc2"]
	assert.False(t, ok, "svc2 should not be present when it has no values")
}

func TestCreateServicesMap_WindowsPathsConvertedToForwardSlashes(t *testing.T) {
	context := &domain.ConfigurationContext{
		Services: []domain.Service{
			{Name: "test-service", Path: `C:\Users\developer\projects\test-service`, GitRef: "main"},
		},
	}

	result := createServicesMap(context)

	svc, ok := result["test-service"].(map[string]interface{})
	require.True(t, ok, "test-service should be present")
	assert.Equal(t, "C:/Users/developer/projects/test-service", svc["path"])
}
