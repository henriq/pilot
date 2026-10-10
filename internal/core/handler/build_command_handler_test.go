package handler

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const buildTestHome = "/home/test"

// mockRawConfig stubs the config repository to resolve context for real, so
// tests assert on the paths and refs domain.ResolveContext actually derives
// rather than on a hand-written stub of them.
func mockRawConfig(configRepository *testutil.MockConfigRepository, context *domain.ConfigurationContext) {
	resolve := func(overrides domain.ContextOverrides) (*domain.ConfigurationContext, error) {
		resolved, err := domain.ResolveContext(*context, buildTestHome, overrides)
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	}
	configRepository.On("LoadConfig").
		Return(&domain.Config{Contexts: []domain.ConfigurationContext{*context}}, nil).Maybe()
	configRepository.On("LoadCurrentContextName").Return(context.Name, nil).Maybe()
	configRepository.On("ResolveCurrentConfigurationContext", mock.Anything).Return(resolve, nil).Maybe()
	configRepository.On("LoadCurrentConfigurationContext").
		Return(resolve, nil).Maybe()
}

func newBuildHandler(
	configRepository *testutil.MockConfigRepository,
	scm *testutil.MockScm,
	containerImageRepository *testutil.MockContainerImageRepository,
) BuildCommandHandler {
	return NewBuildCommandHandler(configRepository, scm, containerImageRepository)
}

func resolvedImagePath(serviceName, gitRepoPath, gitRef string) string {
	return filepath.Join(buildTestHome, ".pilot", "ctx", serviceName, domain.ShortPathHash(gitRepoPath, gitRef))
}

func TestBuildCommandHandler_HandleBuildsAllServices(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "any-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "any-branch"},
				},
				Profiles:     []string{"all"},
				RemoteImages: []string{"remote-1"},
			},
			{
				Name: "service-2",
				DockerImages: []domain.DockerImage{
					{Name: "any-image-2", DockerfilePath: ".", GitRepoPath: "any-repo-2", GitRef: "any-branch-2"},
				},
				Profiles:     []string{"all"},
				RemoteImages: []string{"remote-2"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", "any-repo", "any-branch", resolvedImagePath("service-1", "any-repo", "any-branch")).Return(nil)
	scm.On("Download", "any-repo-2", "any-branch-2", resolvedImagePath("service-2", "any-repo-2", "any-branch-2")).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)
	containerImageRepository.On("PullImage", "remote-1").Return(nil)
	containerImageRepository.On("PullImage", "remote-2").Return(nil)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 2)
}

func TestBuildCommandHandler_HandleBuildsOnlySelectedService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "service-1",
				DockerImages: []domain.DockerImage{{Name: "any-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "any-branch"}},
				Profiles:     []string{"all"},
			},
			{
				Name:         "service-2",
				DockerImages: []domain.DockerImage{{Name: "any-image-2", DockerfilePath: ".", GitRepoPath: "any-repo-2", GitRef: "any-branch-2"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", "any-repo", "any-branch", resolvedImagePath("service-1", "any-repo", "any-branch")).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{"service-1"}, "default", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	scm.AssertNumberOfCalls(t, "Download", 1)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 1)
}

func TestBuildCommandHandler_HandleBuildsOnlyServicesInSelectedProfile(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "service-1",
				DockerImages: []domain.DockerImage{{Name: "any-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "any-branch"}},
				Profiles:     []string{"selected", "all"},
			},
			{
				Name:         "service-2",
				DockerImages: []domain.DockerImage{{Name: "any-image-2", DockerfilePath: ".", GitRepoPath: "any-repo-2", GitRef: "any-branch-2"}},
				Profiles:     []string{"not-selected", "all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", "any-repo", "any-branch", resolvedImagePath("service-1", "any-repo", "any-branch")).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "selected", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	scm.AssertNumberOfCalls(t, "Download", 1)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 1)
}

func TestBuildCommandHandler_Handle_DownloadError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "service-1",
				DockerImages: []domain.DockerImage{{Name: "any-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "any-branch"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.ErrorIs(t, result, assert.AnError)
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything, mock.Anything)
}

func TestBuildCommandHandler_Handle_BuildImageError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "service-1",
				DockerImages: []domain.DockerImage{{Name: "any-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "any-branch"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(assert.AnError)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.ErrorIs(t, result, assert.AnError)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestBuildCommandHandler_Handle_SkipsDownloadWhenImageSourceOverridden(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "service-1",
				DockerImages: []domain.DockerImage{{Name: "any-image", DockerfilePath: "Dockerfile", BuildContextRelativePath: "sub", GitRepoPath: "any-repo", GitRef: "any-branch"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.MatchedBy(func(image domain.DockerImage) bool {
		return image.Name == "any-image" && image.Path == "/local/checkout"
	}), mock.Anything).Return(nil)

	overrides := domain.NewDockerImageSourceOverrides(map[string]string{"any-image": "/local/checkout"})

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", overrides, domain.ServiceGitRefOverrides{})

	assert.Nil(t, result)
	scm.AssertNotCalled(t, "Download", mock.Anything, mock.Anything, mock.Anything)
	containerImageRepository.AssertExpectations(t)
}

func TestBuildCommandHandler_Handle_OverridesOneImageInMultiImageService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "overridden-image", DockerfilePath: ".", GitRepoPath: "r1", GitRef: "b1"},
					{Name: "untouched-image", DockerfilePath: ".", GitRepoPath: "r2", GitRef: "b2"},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", "r2", "b2", resolvedImagePath("service-1", "r2", "b2")).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	overrides := domain.NewDockerImageSourceOverrides(map[string]string{"overridden-image": "/local"})

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", overrides, domain.ServiceGitRefOverrides{})

	assert.Nil(t, result)
	scm.AssertNumberOfCalls(t, "Download", 1)
	scm.AssertNotCalled(t, "Download", "r1", "b1", mock.Anything)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 2)
}

func TestBuildCommandHandler_Handle_RejectsUnknownImageSourceOverride(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "service-1",
				DockerImages: []domain.DockerImage{{Name: "any-image", DockerfilePath: ".", GitRepoPath: "r", GitRef: "b"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	overrides := domain.NewDockerImageSourceOverrides(map[string]string{"unknown-image": "/local"})

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", overrides, domain.ServiceGitRefOverrides{})

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "unknown-image")
	scm.AssertNotCalled(t, "Download", mock.Anything, mock.Anything, mock.Anything)
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything, mock.Anything)
}

func TestBuildCommandHandler_Handle_RejectsUnknownGitRefOverrideService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "api",
				DockerImages: []domain.DockerImage{{Name: "api-image", DockerfilePath: ".", GitRepoPath: "r", GitRef: "main"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	overrides, err := domain.NewServiceGitRefOverrides(map[string]string{"unknown-service": "main"})
	require.NoError(t, err)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, overrides)

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "unknown-service")
	scm.AssertNotCalled(t, "Download", mock.Anything, mock.Anything, mock.Anything)
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything, mock.Anything)
}

func TestBuildCommandHandler_Handle_ThreadsGitRefOverrideToDownloadAndBuild(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "api",
				DockerImages: []domain.DockerImage{{Name: "api-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "main"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)

	overriddenRef := "feature/x"
	expectedPath := resolvedImagePath("api", "any-repo", overriddenRef)
	scm := new(testutil.MockScm)
	scm.On("Download", "any-repo", overriddenRef, expectedPath).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.MatchedBy(func(image domain.DockerImage) bool {
		return image.Name == "api-image" && image.GitRef == overriddenRef && image.Path == expectedPath
	}), mock.Anything).Return(nil)

	overrides, err := domain.NewServiceGitRefOverrides(map[string]string{"api": overriddenRef})
	require.NoError(t, err)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, overrides)

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestBuildCommandHandler_Handle_WarnsWhenImageSourceOverrideOutOfScope(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "in-scope",
				DockerImages: []domain.DockerImage{{Name: "in-scope-image", DockerfilePath: ".", GitRepoPath: "r1", GitRef: "b1"}},
				Profiles:     []string{"selected", "all"},
			},
			{
				Name:         "out-of-scope",
				DockerImages: []domain.DockerImage{{Name: "out-of-scope-image", DockerfilePath: ".", GitRepoPath: "r2", GitRef: "b2"}},
				Profiles:     []string{"not-selected", "all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", "r1", "b1", mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	overrides := domain.NewDockerImageSourceOverrides(map[string]string{"out-of-scope-image": "/local"})

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	stderr := captureStderr(t, func() {
		result := sut.Handle([]string{}, "selected", overrides, domain.ServiceGitRefOverrides{})
		assert.Nil(t, result)
	})

	assert.Contains(t, stderr, `--image-source override for "out-of-scope-image": no matching image in scope; ignoring`)
	scm.AssertNumberOfCalls(t, "Download", 1)
}

func TestBuildCommandHandler_Handle_WarnsWhenGitRefOverrideOutOfScope(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "in-scope",
				DockerImages: []domain.DockerImage{{Name: "in-scope-image", DockerfilePath: ".", GitRepoPath: "r1", GitRef: "main"}},
				Profiles:     []string{"selected", "all"},
			},
			{
				Name:         "out-of-scope",
				DockerImages: []domain.DockerImage{{Name: "out-of-scope-image", DockerfilePath: ".", GitRepoPath: "r2", GitRef: "main"}},
				Profiles:     []string{"not-selected", "all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", "r1", "main", mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	overrides, err := domain.NewServiceGitRefOverrides(map[string]string{"out-of-scope": "feature/x"})
	require.NoError(t, err)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	stderr := captureStderr(t, func() {
		result := sut.Handle([]string{}, "selected", domain.DockerImageSourceOverrides{}, overrides)
		assert.Nil(t, result)
	})

	assert.Contains(t, stderr, `--git-ref override for "out-of-scope": no matching service in scope; ignoring`)
	scm.AssertNumberOfCalls(t, "Download", 1)
}

func TestBuildCommandHandler_HandleIncludesNonDeployableService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "deployable",
				HelmRepoPath: "any-repo",
				HelmBranch:   "any-branch",
				DockerImages: []domain.DockerImage{{Name: "deployable-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "any-branch"}},
				Profiles:     []string{"all"},
			},
			{
				Name:         "automation",
				DockerImages: []domain.DockerImage{{Name: "automation-image", DockerfilePath: ".", GitRepoPath: "any-automation-repo", GitRef: "any-automation-branch"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle(nil, "all", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.Nil(t, result)
	containerImageRepository.AssertNumberOfCalls(t, "BuildImage", 2)
}

func TestBuildCommandHandler_Handle_DoesNotMutateCachedRawConfig(t *testing.T) {
	configContext := domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "api",
				DockerImages: []domain.DockerImage{{Name: "api-image", DockerfilePath: ".", GitRepoPath: "any-repo", GitRef: "main"}},
				Profiles:     []string{"all"},
			},
		},
	}
	rawConfig := &domain.Config{Contexts: []domain.ConfigurationContext{configContext}}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("ResolveCurrentConfigurationContext", mock.Anything).Return(
		func(overrides domain.ContextOverrides) (*domain.ConfigurationContext, error) {
			resolved, err := domain.ResolveContext(rawConfig.Contexts[0], buildTestHome, overrides)
			if err != nil {
				return nil, err
			}
			return &resolved, nil
		}, nil,
	)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.Anything).Return(nil)

	overrides, err := domain.NewServiceGitRefOverrides(map[string]string{"api": "feature/x"})
	require.NoError(t, err)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, overrides)
	assert.Nil(t, result)

	cachedImage := rawConfig.Contexts[0].Services[0].DockerImages[0]
	assert.Equal(t, "main", cachedImage.GitRef, "cached raw config git ref must not be mutated by the override")
	assert.Empty(t, cachedImage.Path, "cached raw config path must not be derived into the cache")
}

// captureStderr redirects os.Stderr for the duration of fn and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stderr = writer

	fn()

	require.NoError(t, writer.Close())
	os.Stderr = original

	var buffer bytes.Buffer
	_, copyErr := io.Copy(&buffer, reader)
	require.NoError(t, copyErr)
	return buffer.String()
}

func TestBuildCommandHandler_Handle_ResolveContextError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("ResolveCurrentConfigurationContext", mock.Anything).Return(nil, assert.AnError)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, domain.ServiceGitRefOverrides{})

	assert.ErrorIs(t, result, assert.AnError)
	scm.AssertNotCalled(t, "Download", mock.Anything, mock.Anything, mock.Anything)
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything, mock.Anything)
}

func TestBuildCommandHandler_Handle_RejectsGitRefOverrideForUnknownService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "api",
				DockerImages: []domain.DockerImage{{Name: "api-image", DockerfilePath: ".", GitRepoPath: "repo", GitRef: "main"}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	overrides, err := domain.NewServiceGitRefOverrides(map[string]string{"nope": "feature/x"})
	require.NoError(t, err)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, overrides)

	require.Error(t, result)
	assert.Contains(t, result.Error(), "service(s) not found")
	assert.Contains(t, result.Error(), "nope")
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything, mock.Anything)
}

func TestBuildCommandHandler_Handle_PassesResolvedContextToBuildImage(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Name: "ctx",
		Services: []domain.Service{
			{
				Name:         "api",
				GitRepoPath:  "repo",
				GitRef:       "main",
				DockerImages: []domain.DockerImage{{Name: "api-image", DockerfilePath: "."}},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	mockRawConfig(configRepository, configContext)
	scm := new(testutil.MockScm)
	scm.On("Download", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", mock.Anything, mock.MatchedBy(func(context *domain.ConfigurationContext) bool {
		return context.Services[0].GitRef == "feature/x"
	})).Return(nil)

	overrides, err := domain.NewServiceGitRefOverrides(map[string]string{"api": "feature/x"})
	require.NoError(t, err)

	sut := newBuildHandler(configRepository, scm, containerImageRepository)

	result := sut.Handle([]string{}, "all", domain.DockerImageSourceOverrides{}, overrides)

	assert.Nil(t, result)
	containerImageRepository.AssertExpectations(t)
}
