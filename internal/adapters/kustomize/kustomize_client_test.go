package kustomize

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"pilot/internal/ports"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCommandRunner struct {
	runFunc func(name string, args ...string) ([]byte, error)
}

func (m *mockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	if m.runFunc != nil {
		return m.runFunc(name, args...)
	}
	return nil, nil
}

func (m *mockCommandRunner) RunWithEnv(name string, env []string, args ...string) ([]byte, error) {
	return nil, nil
}

func (m *mockCommandRunner) RunInDir(dir, name string, args ...string) ([]byte, error) {
	return nil, nil
}

func (m *mockCommandRunner) RunWithEnvInDir(dir string, env []string, name string, args ...string) ([]byte, error) {
	return nil, nil
}

func (m *mockCommandRunner) RunWithStdin(stdin io.Reader, name string, args ...string) ([]byte, error) {
	return nil, nil
}

func (m *mockCommandRunner) RunInteractive(name string, args ...string) error {
	return nil
}

func TestClient_Apply_NoPatches(t *testing.T) {
	runner := &mockCommandRunner{}
	fs := testutil.NewTestFileSystem(t)
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\n")
	result, err := client.Apply(manifests, nil, t.TempDir())

	require.NoError(t, err)
	assert.Equal(t, manifests, result)
}

func TestClient_Apply_CallsKubectl(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	runner := &mockCommandRunner{
		runFunc: func(name string, args ...string) ([]byte, error) {
			capturedName = name
			capturedArgs = args
			return []byte("apiVersion: v1\nkind: ConfigMap\n"), nil
		},
	}
	fs := testutil.NewTestFileSystem(t)
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")
	patches := []ports.Patch{
		{
			Target: ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{
				{Op: "add", Path: "/metadata/annotations/foo", Value: "bar"},
			},
		},
	}

	workDir := t.TempDir()
	_, err := client.Apply(manifests, patches, workDir)
	require.NoError(t, err)

	assert.Equal(t, "kubectl", capturedName)
	assert.Equal(t, "kustomize", capturedArgs[0])
	assert.True(t, strings.HasPrefix(capturedArgs[1], "/"))
}

func TestBuildKustomization_Labels(t *testing.T) {
	patches := []ports.Patch{}

	k, patchFiles, err := buildKustomization(patches)

	require.NoError(t, err)
	assert.Equal(t, "kustomize.config.k8s.io/v1beta1", k.APIVersion)
	assert.Equal(t, "Kustomization", k.Kind)
	require.Len(t, k.Labels, 1)
	assert.Equal(t, "pilot", k.Labels[0].Pairs["managed-by"])
	assert.False(t, k.Labels[0].IncludeSelectors)
	assert.Empty(t, patchFiles)
}

func TestBuildKustomization_AddOperationUsesStrategicMerge(t *testing.T) {
	patches := []ports.Patch{
		{
			Target: ports.PatchTarget{Kind: "Deployment", Name: "my-app"},
			Operations: []ports.PatchOperation{
				{Op: "add", Path: "/spec/template/metadata/annotations/foo", Value: "bar"},
			},
		},
	}

	k, patchFiles, err := buildKustomization(patches)

	require.NoError(t, err)
	require.Len(t, k.Patches, 1)
	assert.Equal(t, "Deployment", k.Patches[0].Target.Kind)
	assert.Equal(t, "my-app", k.Patches[0].Target.Name)
	// Strategic merge uses file reference, not inline patch
	assert.Equal(t, "patch-foo.yaml", k.Patches[0].Path)
	assert.Empty(t, k.Patches[0].Patch)

	// Check patch file content
	require.Len(t, patchFiles, 1)
	patchContent := string(patchFiles[0].Content)
	assert.Contains(t, patchContent, "apiVersion: apps/v1")
	assert.Contains(t, patchContent, "kind: Deployment")
	assert.Contains(t, patchContent, "spec:")
	assert.Contains(t, patchContent, "annotations:")
	assert.Contains(t, patchContent, "foo: bar")
}

func TestBuildKustomization_ReplaceOperationUsesJSONPatch(t *testing.T) {
	patches := []ports.Patch{
		{
			Target: ports.PatchTarget{Kind: "Service", Name: "my-svc"},
			Operations: []ports.PatchOperation{
				{Op: "replace", Path: "/spec/selector/app", Value: "dev-proxy"},
				{Op: "replace", Path: "/spec/ports/0/targetPort", Value: 18080},
			},
		},
	}

	k, patchFiles, err := buildKustomization(patches)

	require.NoError(t, err)
	// JSON patches use inline Patch, no files
	assert.Empty(t, patchFiles)
	require.Len(t, k.Patches, 1)
	assert.Equal(t, "Service", k.Patches[0].Target.Kind)
	assert.Equal(t, "my-svc", k.Patches[0].Target.Name)
	assert.Contains(t, k.Patches[0].Patch, "op: replace")
	assert.Contains(t, k.Patches[0].Patch, "path: /spec/selector/app")
	assert.Contains(t, k.Patches[0].Patch, "value: dev-proxy")
}

func TestBuildKustomization_RemoveOperation(t *testing.T) {
	patches := []ports.Patch{
		{
			Target: ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{
				{Op: "remove", Path: "/data/unwanted"},
			},
		},
	}

	k, patchFiles, err := buildKustomization(patches)

	require.NoError(t, err)
	assert.Empty(t, patchFiles)
	require.Len(t, k.Patches, 1)
	assert.Contains(t, k.Patches[0].Patch, "op: remove")
	assert.Contains(t, k.Patches[0].Patch, "path: /data/unwanted")
}

func TestBuildKustomization_MixedOperations(t *testing.T) {
	patches := []ports.Patch{
		{
			Target: ports.PatchTarget{Kind: "Deployment"},
			Operations: []ports.PatchOperation{
				{Op: "add", Path: "/spec/template/metadata/annotations/foo", Value: "bar"},
				{Op: "replace", Path: "/spec/replicas", Value: 3},
			},
		},
	}

	k, patchFiles, err := buildKustomization(patches)

	require.NoError(t, err)
	// Should have two patches: one strategic merge (file), one JSON patch (inline)
	require.Len(t, k.Patches, 2)

	// First is strategic merge (file reference)
	assert.Equal(t, "patch-foo.yaml", k.Patches[0].Path)
	assert.Empty(t, k.Patches[0].Patch)

	// Second is JSON patch (inline)
	assert.Contains(t, k.Patches[1].Patch, "op: replace")

	// Check patch file
	require.Len(t, patchFiles, 1)
	patchContent := string(patchFiles[0].Content)
	assert.Contains(t, patchContent, "annotations:")
}

func TestBuildStrategicMergePatch(t *testing.T) {
	target := ports.PatchTarget{Kind: "Deployment"}
	op := ports.PatchOperation{
		Op:    "add",
		Path:  "/spec/template/metadata/annotations/kubectl.kubernetes.io~1recreatedAt",
		Value: "2024-01-01T00:00:00Z",
	}

	patchBytes, err := buildStrategicMergePatch(target, op)
	require.NoError(t, err)
	patch := string(patchBytes)

	// Should include apiVersion/kind for strategic merge patch file
	assert.Contains(t, patch, "apiVersion: apps/v1")
	assert.Contains(t, patch, "kind: Deployment")
	assert.Contains(t, patch, "spec:")
	assert.Contains(t, patch, "template:")
	assert.Contains(t, patch, "annotations:")
	// The ~1 should be unescaped to /
	assert.Contains(t, patch, "kubectl.kubernetes.io/recreatedAt:")
}

func TestApiVersionForKind(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		// apps/v1
		{"Deployment", "apps/v1"},
		{"StatefulSet", "apps/v1"},
		{"DaemonSet", "apps/v1"},
		{"ReplicaSet", "apps/v1"},
		// v1
		{"Service", "v1"},
		{"ConfigMap", "v1"},
		{"Secret", "v1"},
		{"Namespace", "v1"},
		{"ServiceAccount", "v1"},
		{"PersistentVolumeClaim", "v1"},
		// networking.k8s.io/v1
		{"Ingress", "networking.k8s.io/v1"},
		// batch/v1
		{"Job", "batch/v1"},
		{"CronJob", "batch/v1"},
		// rbac.authorization.k8s.io/v1
		{"Role", "rbac.authorization.k8s.io/v1"},
		{"RoleBinding", "rbac.authorization.k8s.io/v1"},
		{"ClusterRole", "rbac.authorization.k8s.io/v1"},
		{"ClusterRoleBinding", "rbac.authorization.k8s.io/v1"},
		// default
		{"UnknownKind", "v1"},
		{"CustomResource", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := apiVersionForKind(tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_Apply_KubectlError(t *testing.T) {
	runner := &mockCommandRunner{
		runFunc: func(name string, args ...string) ([]byte, error) {
			return []byte("error: something failed"), fmt.Errorf("exit status 1")
		},
	}
	fs := testutil.NewTestFileSystem(t)
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{{Op: "add", Path: "/data/key", Value: "val"}},
		},
	}

	_, err := client.Apply(manifests, patches, t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl kustomize failed")
}

func TestClient_Apply_WritesCorrectFiles(t *testing.T) {
	runner := &mockCommandRunner{
		runFunc: func(name string, args ...string) ([]byte, error) {
			return []byte("patched output"), nil
		},
	}
	fs := testutil.NewTestFileSystem(t)
	client := NewClient(runner, fs)
	workDir := "workdir"

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{{Op: "add", Path: "/data/key", Value: "val"}},
		},
	}

	_, err := client.Apply(manifests, patches, workDir)
	require.NoError(t, err)

	// Verify resources.yaml was written
	resourcesContent, err := fs.ReadFile(filepath.Join(workDir, "resources.yaml"))
	require.NoError(t, err)
	assert.Equal(t, manifests, resourcesContent)

	// Verify kustomization.yaml exists and is valid
	kustomizationContent, err := fs.ReadFile(filepath.Join(workDir, "kustomization.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(kustomizationContent), "kustomize.config.k8s.io/v1beta1")
	assert.Contains(t, string(kustomizationContent), "managed-by")

	// Verify patch file was created (named after the last path segment "key")
	patchContent, err := fs.ReadFile(filepath.Join(workDir, "patch-key.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(patchContent), "kind: ConfigMap")
	assert.Contains(t, string(patchContent), "data:")
}

func TestBuildKustomization_EmptyOperations(t *testing.T) {
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "Deployment"},
			Operations: []ports.PatchOperation{}, // Empty
		},
	}

	k, patchFiles, err := buildKustomization(patches)

	require.NoError(t, err)
	// Should produce no patches when operations are empty
	assert.Empty(t, k.Patches)
	assert.Empty(t, patchFiles)
}

func TestPatchNameFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/spec/template/metadata/annotations/kubectl.kubernetes.io~1recreatedAt", "recreated-at"},
		{"/metadata/annotations/foo", "foo"},
		{"/data/key", "key"},
		{"/spec/replicas", "replicas"},
		{"/spec/template/metadata/labels/appVersion", "app-version"},
		{"", "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := patchNameFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnescapeJSONPointer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no escapes", "foo", "foo"},
		{"tilde-1 to slash", "kubectl.kubernetes.io~1restartedAt", "kubectl.kubernetes.io/restartedAt"},
		{"tilde-0 to tilde", "annotation~0key", "annotation~key"},
		{"both escapes", "~0~1", "~/"},
		{"multiple tilde-1", "a~1b~1c", "a/b/c"},
		{"order matters - tilde-1 before tilde-0", "~01", "~1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unescapeJSONPointer(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStrategicMergePatch_TildeZeroEscaping(t *testing.T) {
	target := ports.PatchTarget{Kind: "ConfigMap"}
	op := ports.PatchOperation{
		Op:    "add",
		Path:  "/data/key~0with~0tildes",
		Value: "value",
	}

	patchBytes, err := buildStrategicMergePatch(target, op)
	require.NoError(t, err)
	patch := string(patchBytes)

	// ~0 should be unescaped to ~
	assert.Contains(t, patch, "key~with~tildes:")
}

// mockFileSystemWithErrors allows injecting errors for specific operations
type mockFileSystemWithErrors struct {
	mkdirAllErr  error
	writeFileErr func(path string) error
}

func newMockFileSystemWithErrors() *mockFileSystemWithErrors {
	return &mockFileSystemWithErrors{}
}

func (m *mockFileSystemWithErrors) MkdirAll(path string, mode ports.AccessMode) error {
	if m.mkdirAllErr != nil {
		return m.mkdirAllErr
	}
	return nil
}

func (m *mockFileSystemWithErrors) WriteFile(path string, content []byte, mode ports.AccessMode) error {
	if m.writeFileErr != nil {
		if err := m.writeFileErr(path); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockFileSystemWithErrors) ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func (m *mockFileSystemWithErrors) FileExists(path string) (bool, error) {
	return false, nil
}

func (m *mockFileSystemWithErrors) EnsureDirExists(path string) error {
	return nil
}

func (m *mockFileSystemWithErrors) RemoveAll(path string) error {
	return nil
}

func (m *mockFileSystemWithErrors) HomeDir() (string, error) {
	return "/home/test", nil
}

func TestClient_Apply_MkdirAllError(t *testing.T) {
	runner := &mockCommandRunner{}
	fs := newMockFileSystemWithErrors()
	fs.mkdirAllErr = fmt.Errorf("permission denied")
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\n")
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{{Op: "add", Path: "/data/key", Value: "val"}},
		},
	}

	_, err := client.Apply(manifests, patches, "/workdir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create work directory")
}

func TestClient_Apply_WriteResourcesError(t *testing.T) {
	runner := &mockCommandRunner{}
	fs := newMockFileSystemWithErrors()
	fs.writeFileErr = func(path string) error {
		if strings.HasSuffix(path, "resources.yaml") {
			return fmt.Errorf("disk full")
		}
		return nil
	}
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\n")
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{{Op: "add", Path: "/data/key", Value: "val"}},
		},
	}

	_, err := client.Apply(manifests, patches, "/workdir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write resources")
}

func TestClient_Apply_WritePatchFileError(t *testing.T) {
	runner := &mockCommandRunner{}
	fs := newMockFileSystemWithErrors()
	fs.writeFileErr = func(path string) error {
		if strings.HasPrefix(filepath.Base(path), "patch-") {
			return fmt.Errorf("disk full")
		}
		return nil
	}
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\n")
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{{Op: "add", Path: "/data/key", Value: "val"}},
		},
	}

	_, err := client.Apply(manifests, patches, "/workdir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write patch file")
}

func TestClient_Apply_WriteKustomizationError(t *testing.T) {
	runner := &mockCommandRunner{}
	fs := newMockFileSystemWithErrors()
	fs.writeFileErr = func(path string) error {
		if strings.HasSuffix(path, "kustomization.yaml") {
			return fmt.Errorf("disk full")
		}
		return nil
	}
	client := NewClient(runner, fs)

	manifests := []byte("apiVersion: v1\nkind: ConfigMap\n")
	patches := []ports.Patch{
		{
			Target:     ports.PatchTarget{Kind: "ConfigMap"},
			Operations: []ports.PatchOperation{{Op: "add", Path: "/data/key", Value: "val"}},
		},
	}

	_, err := client.Apply(manifests, patches, "/workdir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write kustomization.yaml")
}
