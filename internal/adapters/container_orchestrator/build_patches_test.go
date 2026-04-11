package container_orchestrator

import (
	"testing"
	"time"

	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetes_BuildPatches_WithoutInterceptHttp(t *testing.T) {
	mockConfigRepo := new(testutil.MockConfigRepository)
	mockConfigRepo.On("LoadCurrentConfigurationContext").Return(&domain.ConfigurationContext{
		LocalServices: []domain.LocalService{
			{Name: "service-a", LocalPort: 3000, KubernetesPort: 80, HealthCheckPath: "/health"},
		},
	}, nil)

	sut := &Kubernetes{configRepository: mockConfigRepo}

	patches, err := sut.buildPatches(false)

	require.NoError(t, err)
	require.Len(t, patches, 2)

	// First patch: Deployment annotation
	assert.Equal(t, "Deployment", patches[0].Target.Kind)
	assert.Equal(t, "", patches[0].Target.Name)
	require.Len(t, patches[0].Operations, 1)
	assert.Equal(t, "add", patches[0].Operations[0].Op)
	assert.Contains(t, patches[0].Operations[0].Path, "recreatedAt")

	// Verify the timestamp is a valid RFC3339 time close to now
	timestamp, ok := patches[0].Operations[0].Value.(string)
	require.True(t, ok, "annotation value should be a string")
	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	require.NoError(t, err, "annotation value should be valid RFC3339")
	assert.WithinDuration(t, time.Now(), parsedTime, 5*time.Second)

	// Second patch: Service patch with HAProxy port (8080) since interceptHttp=false
	assert.Equal(t, "Service", patches[1].Target.Kind)
	assert.Equal(t, "service-a", patches[1].Target.Name)
	require.Len(t, patches[1].Operations, 2)
	assert.Equal(t, "replace", patches[1].Operations[0].Op)
	assert.Equal(t, "/spec/selector/app", patches[1].Operations[0].Path)
	assert.Equal(t, "dev-proxy", patches[1].Operations[0].Value)
	assert.Equal(t, "replace", patches[1].Operations[1].Op)
	assert.Equal(t, "/spec/ports/0/targetPort", patches[1].Operations[1].Path)
	assert.Equal(t, core.DevProxyHAProxyStartPort, patches[1].Operations[1].Value)

	mockConfigRepo.AssertExpectations(t)
}

func TestKubernetes_BuildPatches_WithInterceptHttp(t *testing.T) {
	mockConfigRepo := new(testutil.MockConfigRepository)
	mockConfigRepo.On("LoadCurrentConfigurationContext").Return(&domain.ConfigurationContext{
		LocalServices: []domain.LocalService{
			{Name: "service-a", LocalPort: 3000, KubernetesPort: 80, HealthCheckPath: "/health"},
		},
	}, nil)

	sut := &Kubernetes{configRepository: mockConfigRepo}

	patches, err := sut.buildPatches(true)

	require.NoError(t, err)
	require.Len(t, patches, 2)

	// Second patch: Service patch with mitmproxy port (18080) since interceptHttp=true
	servicePatch := patches[1]
	assert.Equal(t, "Service", servicePatch.Target.Kind)
	assert.Equal(t, "service-a", servicePatch.Target.Name)
	require.Len(t, servicePatch.Operations, 2)
	assert.Equal(t, "replace", servicePatch.Operations[0].Op)
	assert.Equal(t, "/spec/selector/app", servicePatch.Operations[0].Path)
	assert.Equal(t, "dev-proxy", servicePatch.Operations[0].Value)
	assert.Equal(t, "replace", servicePatch.Operations[1].Op)
	assert.Equal(t, "/spec/ports/0/targetPort", servicePatch.Operations[1].Path)
	assert.Equal(t, core.DevProxyMitmproxyStartPort, servicePatch.Operations[1].Value)

	mockConfigRepo.AssertExpectations(t)
}

func TestKubernetes_BuildPatches_PortIncrements(t *testing.T) {
	mockConfigRepo := new(testutil.MockConfigRepository)
	mockConfigRepo.On("LoadCurrentConfigurationContext").Return(&domain.ConfigurationContext{
		LocalServices: []domain.LocalService{
			{Name: "frontend", LocalPort: 3000, KubernetesPort: 80, HealthCheckPath: "/health"},
			{Name: "backend", LocalPort: 4000, KubernetesPort: 80, HealthCheckPath: "/api/health"},
			{Name: "worker", LocalPort: 5000, KubernetesPort: 80, HealthCheckPath: "/ready"},
		},
	}, nil)

	sut := &Kubernetes{configRepository: mockConfigRepo}

	patches, err := sut.buildPatches(false)

	require.NoError(t, err)
	// 1 Deployment patch + 3 Service patches
	require.Len(t, patches, 4)

	// Verify each service gets a sequential port starting from HAProxy start port
	expectedServices := []struct {
		name string
		port int
	}{
		{"frontend", core.DevProxyHAProxyStartPort},
		{"backend", core.DevProxyHAProxyStartPort + 1},
		{"worker", core.DevProxyHAProxyStartPort + 2},
	}

	for i, expected := range expectedServices {
		patch := patches[i+1] // skip the Deployment patch at index 0
		assert.Equal(t, "Service", patch.Target.Kind)
		assert.Equal(t, expected.name, patch.Target.Name)
		require.Len(t, patch.Operations, 2)
		assert.Equal(t, expected.port, patch.Operations[1].Value,
			"service %q should get port %d", expected.name, expected.port)
	}

	mockConfigRepo.AssertExpectations(t)
}

func TestKubernetes_BuildPatches_LoadConfigError(t *testing.T) {
	expectedErr := assert.AnError
	mockConfigRepo := new(testutil.MockConfigRepository)
	mockConfigRepo.On("LoadCurrentConfigurationContext").Return(nil, expectedErr)

	sut := &Kubernetes{configRepository: mockConfigRepo}

	patches, err := sut.buildPatches(false)

	assert.Nil(t, patches)
	assert.ErrorIs(t, err, expectedErr)

	mockConfigRepo.AssertExpectations(t)
}
