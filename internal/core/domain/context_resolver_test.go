package domain

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const resolverTestHome = "/home/testuser"

func mustGitRefOverrides(t *testing.T, gitRefByService map[string]string) ServiceGitRefOverrides {
	t.Helper()
	overrides, err := NewServiceGitRefOverrides(gitRefByService)
	require.NoError(t, err)
	return overrides
}

func expectedImagePath(contextName, serviceName, gitRepoPath, gitRef string) string {
	return filepath.Join(resolverTestHome, ".pilot", contextName, serviceName, ShortPathHash(gitRepoPath, gitRef))
}

func expectedHelmPath(contextName, helmRepoPath, helmBranch string) string {
	return filepath.Join(resolverTestHome, ".pilot", contextName, "charts", ShortPathHash(helmRepoPath, helmBranch))
}

func deployableService() Service {
	return Service{
		Name:                  "api",
		HelmRepoPath:          "helm-repo",
		HelmBranch:            "helm-main",
		HelmChartRelativePath: "charts/api",
		GitRepoPath:           "svc-repo",
		GitRef:                "svc-main",
		DockerImages: []DockerImage{
			{Name: "api-image", DockerfilePath: "Dockerfile", BuildContextRelativePath: "."},
		},
	}
}

func TestResolveContext_DerivesPathsAndInheritsGitRef(t *testing.T) {
	raw := ConfigurationContext{Name: "ctx", Services: []Service{deployableService()}}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	service := resolved.Services[0]
	image := service.DockerImages[0]

	assert.Equal(t, "svc-repo", image.GitRepoPath, "image inherits the service git repo")
	assert.Equal(t, "svc-main", image.GitRef, "image inherits the service git ref")
	assert.Equal(t, expectedImagePath("ctx", "api", "svc-repo", "svc-main"), image.Path)
	assert.Equal(t, expectedHelmPath("ctx", "helm-repo", "helm-main"), service.HelmPath)
	assert.Equal(t, expectedImagePath("ctx", "api", "svc-repo", "svc-main"), service.Path)
	assert.Equal(t, []string{"default", "all"}, service.Profiles)
}

func TestResolveContext_KeepsExplicitImageGitRef(t *testing.T) {
	service := deployableService()
	service.DockerImages[0].GitRepoPath = "image-repo"
	service.DockerImages[0].GitRef = "image-ref"
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	image := resolved.Services[0].DockerImages[0]
	assert.Equal(t, "image-repo", image.GitRepoPath)
	assert.Equal(t, "image-ref", image.GitRef)
	assert.Equal(t, expectedImagePath("ctx", "api", "image-repo", "image-ref"), image.Path)
}

func TestResolveContext_GitRefOverrideAppliesToAllImagesOfTargetedServiceOnly(t *testing.T) {
	api := deployableService()
	api.DockerImages = []DockerImage{
		{Name: "api-inheriting", DockerfilePath: "Dockerfile", BuildContextRelativePath: "."},
		{Name: "api-pinned", DockerfilePath: "Dockerfile", BuildContextRelativePath: ".", GitRepoPath: "pinned-repo", GitRef: "pinned-ref"},
	}
	worker := deployableService()
	worker.Name = "worker"
	worker.GitRepoPath = "worker-repo"
	worker.GitRef = "worker-main"
	worker.DockerImages = []DockerImage{
		{Name: "worker-image", DockerfilePath: "Dockerfile", BuildContextRelativePath: "."},
	}
	raw := ConfigurationContext{Name: "ctx", Services: []Service{api, worker}}

	overrides := ContextOverrides{ServiceGitRefs: mustGitRefOverrides(t, map[string]string{"api": "feature/x"})}
	resolved, err := ResolveContext(raw, resolverTestHome, overrides)
	require.NoError(t, err)

	inheriting := resolved.Services[0].DockerImages[0]
	assert.Equal(t, "feature/x", inheriting.GitRef)
	assert.Equal(t, expectedImagePath("ctx", "api", "svc-repo", "feature/x"), inheriting.Path)

	pinned := resolved.Services[0].DockerImages[1]
	assert.Equal(t, "feature/x", pinned.GitRef, "git-ref override replaces an explicitly pinned ref too")
	assert.Equal(t, expectedImagePath("ctx", "api", "pinned-repo", "feature/x"), pinned.Path)

	assert.Equal(t, "feature/x", resolved.Services[0].GitRef,
		"the override is service-scoped, so templated values and pilot run see the same ref")
	assert.Equal(t, expectedImagePath("ctx", "api", "svc-repo", "feature/x"), resolved.Services[0].Path)

	untouched := resolved.Services[1].DockerImages[0]
	assert.Equal(t, "worker-main", untouched.GitRef, "a sibling service is left untouched")
	assert.Equal(t, expectedImagePath("ctx", "worker", "worker-repo", "worker-main"), untouched.Path)
}

func TestResolveContext_GitRefOverrideOnServiceWithoutImages(t *testing.T) {
	service := deployableService()
	service.DockerImages = nil
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	overrides := ContextOverrides{ServiceGitRefs: mustGitRefOverrides(t, map[string]string{"api": "feature/x"})}
	resolved, err := ResolveContext(raw, resolverTestHome, overrides)

	require.NoError(t, err)
	assert.Empty(t, resolved.Services[0].DockerImages)
}

func TestResolveContext_ImageSourceOverrideSetsPathAndKeepsGitRef(t *testing.T) {
	service := deployableService()
	service.DockerImages = []DockerImage{
		{Name: "api-image", DockerfilePath: "Dockerfile", BuildContextRelativePath: "."},
		{Name: "sidecar-image", DockerfilePath: "Dockerfile", BuildContextRelativePath: "."},
	}
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	overrides := ContextOverrides{ImageSources: NewDockerImageSourceOverrides(map[string]string{"api-image": "/local/api"})}
	resolved, err := ResolveContext(raw, resolverTestHome, overrides)
	require.NoError(t, err)

	overridden := resolved.Services[0].DockerImages[0]
	assert.Equal(t, "/local/api", overridden.Path)
	assert.Equal(t, "svc-main", overridden.GitRef, "image-source leaves the git ref for display")

	sibling := resolved.Services[0].DockerImages[1]
	assert.Equal(t, expectedImagePath("ctx", "api", "svc-repo", "svc-main"), sibling.Path, "sibling image keeps its derived path")
}

func TestResolveContext_HelmChartOverrideSetsPathAndClearsRelativePath(t *testing.T) {
	api := deployableService()
	sibling := deployableService()
	sibling.Name = "web"
	raw := ConfigurationContext{Name: "ctx", Services: []Service{api, sibling}}

	overrides := ContextOverrides{HelmCharts: NewHelmChartOverrides(map[string]string{"api": "/local/chart"})}
	resolved, err := ResolveContext(raw, resolverTestHome, overrides)
	require.NoError(t, err)

	overridden := resolved.Services[0]
	assert.Equal(t, "/local/chart", overridden.HelmPath)
	assert.Empty(t, overridden.HelmChartRelativePath)

	assert.Equal(t, expectedHelmPath("ctx", "helm-repo", "helm-main"), resolved.Services[1].HelmPath, "sibling keeps its derived helm path")
}

func TestResolveContext_ImageSourceWinsOverGitRefForPath(t *testing.T) {
	raw := ConfigurationContext{Name: "ctx", Services: []Service{deployableService()}}

	overrides := ContextOverrides{
		ServiceGitRefs: mustGitRefOverrides(t, map[string]string{"api": "feature/x"}),
		ImageSources:   NewDockerImageSourceOverrides(map[string]string{"api-image": "/local/api"}),
	}
	resolved, err := ResolveContext(raw, resolverTestHome, overrides)
	require.NoError(t, err)

	image := resolved.Services[0].DockerImages[0]
	assert.Equal(t, "feature/x", image.GitRef, "git-ref override still sets the ref")
	assert.Equal(t, "/local/api", image.Path, "image-source override wins for the path")
}

func TestResolveContext_ProfileDefaulting(t *testing.T) {
	tests := []struct {
		name       string
		service    Service
		wantResult []string
	}{
		{
			name:       "deployable without profiles gets default and all",
			service:    Service{Name: "svc", HelmRepoPath: "r", HelmBranch: "b", HelmChartRelativePath: "c"},
			wantResult: []string{"default", "all"},
		},
		{
			name:       "non-deployable without profiles gets all only",
			service:    Service{Name: "svc"},
			wantResult: []string{"all"},
		},
		{
			name:       "deployable with custom profile keeps it and appends all",
			service:    Service{Name: "svc", HelmRepoPath: "r", HelmBranch: "b", HelmChartRelativePath: "c", Profiles: []string{"custom"}},
			wantResult: []string{"custom", "all"},
		},
		{
			name:       "profile all is not duplicated",
			service:    Service{Name: "svc", HelmRepoPath: "r", HelmBranch: "b", HelmChartRelativePath: "c", Profiles: []string{"all"}},
			wantResult: []string{"all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := ConfigurationContext{Name: "ctx", Services: []Service{tt.service}}
			resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
			require.NoError(t, err)
			assert.Equal(t, tt.wantResult, resolved.Services[0].Profiles)
		})
	}
}

func TestResolveContext_NonDeployableServiceHasNoHelmPath(t *testing.T) {
	raw := ConfigurationContext{Name: "ctx", Services: []Service{{Name: "automation"}}}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	assert.Empty(t, resolved.Services[0].HelmPath)
}

func TestResolveContext_ServiceWithoutGitRefHasNoPath(t *testing.T) {
	raw := ConfigurationContext{Name: "ctx", Services: []Service{{Name: "svc", GitRepoPath: "repo"}}}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	assert.Empty(t, resolved.Services[0].Path)
}

func TestResolveContext_RejectsUnknownOverrideTargets(t *testing.T) {
	raw := ConfigurationContext{Name: "ctx", Services: []Service{deployableService()}}

	tests := []struct {
		name      string
		overrides ContextOverrides
		wantIn    string
	}{
		{
			name:      "unknown git-ref service",
			overrides: ContextOverrides{ServiceGitRefs: mustGitRefOverrides(t, map[string]string{"nope": "main"})},
			wantIn:    "nope",
		},
		{
			name:      "unknown image-source image",
			overrides: ContextOverrides{ImageSources: NewDockerImageSourceOverrides(map[string]string{"nope-image": "/local"})},
			wantIn:    "nope-image",
		},
		{
			name:      "unknown helm-chart service",
			overrides: ContextOverrides{HelmCharts: NewHelmChartOverrides(map[string]string{"nope": "/local"})},
			wantIn:    "nope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveContext(raw, resolverTestHome, tt.overrides)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantIn)
			assert.Contains(t, err.Error(), "api", "error lists the available targets")
		})
	}
}

func TestResolveContext_DeepCopyIsolatesMapsAndNestedSlices(t *testing.T) {
	raw := ConfigurationContext{
		Name:          "ctx",
		Scripts:       map[string]string{"seed": "make seed"},
		Services:      []Service{deployableService()},
		LocalServices: []LocalService{{Name: "api", Selector: map[string]string{"app": "api"}}},
	}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	resolved.Scripts["seed"] = "make wiped"
	resolved.LocalServices[0].Selector["app"] = "wiped"

	assert.Equal(t, "make seed", raw.Scripts["seed"], "Scripts map must be deep-copied")
	assert.Equal(t, "api", raw.LocalServices[0].Selector["app"], "LocalService selector must be deep-copied")
}

func TestResolveContext_RejectsMalformedConfigGitRef(t *testing.T) {
	service := deployableService()
	service.GitRef = "--upload-pack=/tmp/x.sh"
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	_, err := ResolveContext(raw, resolverTestHome, NoOverrides)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api")
	assert.Contains(t, err.Error(), "cannot start with '-'")
}

func TestResolveContext_IsDeterministic(t *testing.T) {
	raw := ConfigurationContext{Name: "ctx", Services: []Service{deployableService()}}

	first, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)
	second, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestResolveContext_IsDeterministicWithOverrides(t *testing.T) {
	api := deployableService()
	worker := deployableService()
	worker.Name = "worker"
	raw := ConfigurationContext{Name: "ctx", Services: []Service{api, worker}}
	overrides := ContextOverrides{ServiceGitRefs: mustGitRefOverrides(t, map[string]string{"api": "feature/x", "worker": "hotfix/1"})}

	first, err := ResolveContext(raw, resolverTestHome, overrides)
	require.NoError(t, err)
	second, err := ResolveContext(raw, resolverTestHome, overrides)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestResolveContext_DoesNotMutateRawAcrossCalls(t *testing.T) {
	service := deployableService()
	service.RemoteImages = []string{"postgres:latest"}
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	overridden, err := ResolveContext(raw, resolverTestHome, ContextOverrides{
		ServiceGitRefs: mustGitRefOverrides(t, map[string]string{"api": "feature/x"}),
	})
	require.NoError(t, err)
	assert.Equal(t, "feature/x", overridden.Services[0].DockerImages[0].GitRef)

	rawImage := raw.Services[0].DockerImages[0]
	assert.Empty(t, rawImage.GitRef, "override must not leak into the raw image ref")
	assert.Empty(t, rawImage.Path, "derivation must not leak into the raw image path")
	assert.Empty(t, raw.Services[0].HelmPath, "derivation must not leak into the raw helm path")
	assert.Empty(t, raw.Services[0].Path, "derivation must not leak into the raw service path")
	assert.Nil(t, raw.Services[0].Profiles, "profile defaulting must not leak into the raw config")
	assert.Equal(t, []string{"postgres:latest"}, raw.Services[0].RemoteImages)

	clean, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)
	assert.Equal(t, "svc-main", clean.Services[0].DockerImages[0].GitRef, "a later resolve sees the configured ref, not the earlier override")
	assert.Equal(t, []string{"default", "all"}, clean.Services[0].Profiles, "profiles are not accumulated across calls")
}

func TestResolveContext_RejectsMalformedHelmBranch(t *testing.T) {
	service := deployableService()
	service.HelmBranch = "refs/heads/*:refs/heads/*"
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	_, err := ResolveContext(raw, resolverTestHome, NoOverrides)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid helm branch for service \"api\"")
}

func TestResolveContext_RejectsMalformedServiceGitRefWhenImagePinsItsOwn(t *testing.T) {
	service := deployableService()
	service.GitRef = "main:refs/heads/pwned"
	service.DockerImages = []DockerImage{
		{Name: "api-image", DockerfilePath: "Dockerfile", GitRepoPath: "pinned-repo", GitRef: "pinned-ref"},
	}
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	_, err := ResolveContext(raw, resolverTestHome, NoOverrides)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid git ref for service \"api\"")
}

func TestResolveContext_RejectsMalformedServiceGitRefWhenServiceHasNoImages(t *testing.T) {
	service := deployableService()
	service.GitRef = "--upload-pack=/tmp/x.sh"
	service.DockerImages = nil
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	_, err := ResolveContext(raw, resolverTestHome, NoOverrides)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start with '-'")
}

func TestResolveContext_DeepCopyIsolatesCertificatesAndLocalPort(t *testing.T) {
	localPort := 8080
	secretKeys := OpaqueSecretKeys{}
	service := deployableService()
	service.LocalPort = &localPort
	service.Certificates = []CertificateRequest{
		{DNSNames: []string{"api.test"}, K8sSecret: K8sSecretConfig{Keys: &secretKeys}},
	}
	raw := ConfigurationContext{Name: "ctx", Services: []Service{service}}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	resolved.Services[0].Certificates[0].DNSNames[0] = "wiped"
	*resolved.Services[0].LocalPort = 9999

	assert.Equal(t, "api.test", raw.Services[0].Certificates[0].DNSNames[0], "DNSNames must be deep-copied")
	assert.Equal(t, 8080, *raw.Services[0].LocalPort, "LocalPort must be deep-copied")
	assert.NotSame(t, raw.Services[0].Certificates[0].K8sSecret.Keys, resolved.Services[0].Certificates[0].K8sSecret.Keys,
		"K8sSecret keys must be deep-copied")
}

func TestResolveContext_DeepCopyIsolatesImport(t *testing.T) {
	importPath := "/shared/pilot.yaml"
	raw := ConfigurationContext{Name: "ctx", Import: &importPath, Services: []Service{deployableService()}}

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	*resolved.Import = "wiped"

	assert.Equal(t, "/shared/pilot.yaml", *raw.Import, "Import must be deep-copied")
}
