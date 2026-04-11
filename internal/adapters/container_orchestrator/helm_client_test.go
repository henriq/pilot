package container_orchestrator

import (
	"errors"
	"testing"

	"pilot/internal/ports"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelmClient_Template(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"template", "my-release", "/path/to/chart", "--namespace", "my-namespace", "--set", "foo=bar"}).
		Return([]byte("apiVersion: v1\nkind: ConfigMap\n"), nil)

	client := NewHelmClient(runner)

	output, err := client.Template("my-release", "/path/to/chart", "my-namespace", []string{"--set", "foo=bar"})

	require.NoError(t, err)
	assert.Contains(t, string(output), "ConfigMap")
	runner.AssertExpectations(t)
}

func TestHelmClient_Template_NoNamespace(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"template", "my-release", "/path/to/chart"}).
		Return([]byte(""), nil)

	client := NewHelmClient(runner)

	_, err := client.Template("my-release", "/path/to/chart", "", nil)

	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestHelmClient_Template_Error(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"template", "my-release", "/path/to/chart"}).
		Return([]byte("Error: chart not found"), errors.New("exit status 1"))

	client := NewHelmClient(runner)

	_, err := client.Template("my-release", "/path/to/chart", "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "helm template failed")
	runner.AssertExpectations(t)
}

func TestHelmClient_UpgradeFromManifests(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"upgrade", "--install", "--labels", "managed-by=pilot", "my-release", "/path/to/wrapper", "--namespace", "my-namespace"}).
		Return([]byte("Release \"my-release\" has been upgraded."), nil)

	client := NewHelmClient(runner)

	err := client.UpgradeFromManifests("my-release", "my-namespace", "/path/to/wrapper")

	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestHelmClient_UpgradeFromManifests_NoNamespace(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"upgrade", "--install", "--labels", "managed-by=pilot", "my-release", "/path/to/wrapper"}).
		Return([]byte(""), nil)

	client := NewHelmClient(runner)

	err := client.UpgradeFromManifests("my-release", "", "/path/to/wrapper")

	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestHelmClient_Uninstall(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"uninstall", "my-release", "--namespace", "my-namespace"}).
		Return([]byte(""), nil)

	client := NewHelmClient(runner)

	err := client.Uninstall("my-release", "my-namespace")

	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestHelmClient_Uninstall_NoNamespace(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"uninstall", "my-release"}).
		Return([]byte(""), nil)

	client := NewHelmClient(runner)

	err := client.Uninstall("my-release", "")

	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestHelmClient_List(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"list", "-l", "managed-by=pilot", "--short", "--namespace", "my-namespace"}).
		Return([]byte("release1\nrelease2\nrelease3"), nil)

	client := NewHelmClient(runner)

	releases, err := client.List("managed-by=pilot", "my-namespace")

	require.NoError(t, err)
	assert.Equal(t, []string{"release1", "release2", "release3"}, releases)
	runner.AssertExpectations(t)
}

func TestHelmClient_List_NoNamespace(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"list", "-l", "managed-by=pilot", "--short"}).
		Return([]byte("release1\nrelease2"), nil)

	client := NewHelmClient(runner)

	releases, err := client.List("managed-by=pilot", "")

	require.NoError(t, err)
	assert.Equal(t, []string{"release1", "release2"}, releases)
	runner.AssertExpectations(t)
}

func TestHelmClient_List_Empty(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"list", "-l", "managed-by=pilot", "--short", "--namespace", "my-namespace"}).
		Return([]byte(""), nil)

	client := NewHelmClient(runner)

	releases, err := client.List("managed-by=pilot", "my-namespace")

	require.NoError(t, err)
	assert.Empty(t, releases)
	runner.AssertExpectations(t)
}

func TestHelmClient_UpgradeFromManifests_Error(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"upgrade", "--install", "--labels", "managed-by=pilot", "my-release", "/path/to/wrapper", "--namespace", "my-namespace"}).
		Return([]byte("Error: release failed"), errors.New("exit status 1"))

	client := NewHelmClient(runner)

	err := client.UpgradeFromManifests("my-release", "my-namespace", "/path/to/wrapper")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "helm upgrade failed")
	assert.Contains(t, err.Error(), "release failed")
	runner.AssertExpectations(t)
}

func TestHelmClient_Uninstall_Error(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"uninstall", "my-release", "--namespace", "my-namespace"}).
		Return([]byte("Error: release not found"), errors.New("exit status 1"))

	client := NewHelmClient(runner)

	err := client.Uninstall("my-release", "my-namespace")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to uninstall helm chart")
	assert.Contains(t, err.Error(), "release not found")
	runner.AssertExpectations(t)
}

func TestHelmClient_List_Error(t *testing.T) {
	runner := new(testutil.MockCommandRunner)
	runner.On("Run", "helm", []string{"list", "-l", "managed-by=pilot", "--short", "--namespace", "my-namespace"}).
		Return([]byte("Error: cannot access cluster"), errors.New("exit status 1"))

	client := NewHelmClient(runner)

	_, err := client.List("managed-by=pilot", "my-namespace")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list helm charts")
	assert.Contains(t, err.Error(), "cannot access cluster")
	runner.AssertExpectations(t)
}

func TestHelmClientInterface(t *testing.T) {
	// Verify HelmClient implements the ports.HelmClient interface
	var _ ports.HelmClient = (*HelmClient)(nil)
}
