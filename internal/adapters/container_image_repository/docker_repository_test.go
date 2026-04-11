package container_image_repository

import (
	"errors"
	"io"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupMocks() (*testutil.MockConfigRepository, *testutil.MockSecretsRepository, *testutil.MockTemplater, *testutil.MockCommandRunner) {
	configRepo := new(testutil.MockConfigRepository)
	secretsRepo := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	commandRunner := new(testutil.MockCommandRunner)

	// Default mock setup for CreateTemplatingValues
	configRepo.On("LoadCurrentContextName").Return("test-context", nil)
	configRepo.On("LoadCurrentConfigurationContext").Return(&domain.ConfigurationContext{
		Name: "test-context",
	}, nil)
	secretsRepo.On("LoadSecrets", "test-context").Return([]*domain.Secret{}, nil)

	return configRepo, secretsRepo, templater, commandRunner
}

func TestDockerRepository_BuildImage_Success(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	commandRunner.On("Run", "docker", []string{"build", "-t", "my-image", "-f", "/path/to/repo/Dockerfile", "/path/to/repo"}).
		Return([]byte("Successfully built abc123"), nil)

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	image := domain.DockerImage{
		Name:                     "my-image",
		DockerfilePath:           "Dockerfile",
		BuildContextRelativePath: ".",
		Path:                     "/path/to/repo",
	}

	err := repo.BuildImage(image)

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestDockerRepository_BuildImage_WithBuildArgs(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	templater.On("Render", "--build-arg=VERSION=1.0", "build-args.0", mock.Anything).
		Return("--build-arg=VERSION=1.0", nil)
	templater.On("Render", "--build-arg=ENV=prod", "build-args.1", mock.Anything).
		Return("--build-arg=ENV=prod", nil)

	commandRunner.On("Run", "docker", []string{
		"build", "-t", "my-image", "-f", "/path/to/repo/Dockerfile",
		"--build-arg=VERSION=1.0", "--build-arg=ENV=prod", "/path/to/repo",
	}).Return([]byte("Successfully built"), nil)

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	image := domain.DockerImage{
		Name:                     "my-image",
		DockerfilePath:           "Dockerfile",
		BuildContextRelativePath: ".",
		BuildArgs:                []string{"--build-arg=VERSION=1.0", "--build-arg=ENV=prod"},
		Path:                     "/path/to/repo",
	}

	err := repo.BuildImage(image)

	require.NoError(t, err)
	templater.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
}

func TestDockerRepository_BuildImage_WithDockerfileOverride(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	dockerfileContent := "FROM alpine:latest\nRUN echo hello"

	commandRunner.On("RunWithStdin", mock.Anything, "docker", []string{
		"build", "-t", "my-image", "-f", "-", "/path/to/repo",
	}).Return([]byte("Successfully built"), nil)

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	image := domain.DockerImage{
		Name:                     "my-image",
		DockerfileOverride:       dockerfileContent,
		BuildContextRelativePath: ".",
		Path:                     "/path/to/repo",
	}

	err := repo.BuildImage(image)

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)

	// Verify that stdin was passed with dockerfile content
	call := commandRunner.Calls[0]
	stdin := call.Arguments.Get(0).(io.Reader)
	content, _ := io.ReadAll(stdin)
	assert.Equal(t, dockerfileContent, string(content))
}

func TestDockerRepository_BuildImage_DockerCommandFails(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	commandRunner.On("Run", "docker", mock.Anything).
		Return([]byte("error: unable to prepare context"), errors.New("exit status 1"))

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	image := domain.DockerImage{
		Name:                     "my-image",
		DockerfilePath:           "Dockerfile",
		BuildContextRelativePath: ".",
		Path:                     "/path/to/repo",
	}

	err := repo.BuildImage(image)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build image")
	assert.Contains(t, err.Error(), "unable to prepare context")
}

func TestDockerRepository_BuildImage_TemplateRenderFails(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	templater.On("Render", "--build-arg={{.Missing}}", "build-args.0", mock.Anything).
		Return("", errors.New("template error: Missing not defined"))

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	image := domain.DockerImage{
		Name:                     "my-image",
		DockerfilePath:           "Dockerfile",
		BuildContextRelativePath: ".",
		BuildArgs:                []string{"--build-arg={{.Missing}}"},
		Path:                     "/path/to/repo",
	}

	err := repo.BuildImage(image)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "template error")
}

func TestDockerRepository_BuildImage_ConfigRepoFails(t *testing.T) {
	configRepo := new(testutil.MockConfigRepository)
	secretsRepo := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	commandRunner := new(testutil.MockCommandRunner)

	configRepo.On("LoadCurrentContextName").Return("", errors.New("no context configured"))

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	image := domain.DockerImage{
		Name:                     "my-image",
		DockerfilePath:           "Dockerfile",
		BuildContextRelativePath: ".",
		Path:                     "/path/to/repo",
	}

	err := repo.BuildImage(image)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no context configured")
}

func TestDockerRepository_PullImage_Success(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	commandRunner.On("Run", "docker", []string{"pull", "nginx:latest"}).
		Return([]byte("Status: Downloaded newer image for nginx:latest"), nil)

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	err := repo.PullImage("nginx:latest")

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestDockerRepository_PullImage_Fails(t *testing.T) {
	configRepo, secretsRepo, templater, commandRunner := setupMocks()

	commandRunner.On("Run", "docker", []string{"pull", "nonexistent:image"}).
		Return([]byte("Error: pull access denied"), errors.New("exit status 1"))

	repo := NewDockerRepository(configRepo, secretsRepo, templater, commandRunner)

	err := repo.PullImage("nonexistent:image")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pull image")
	assert.Contains(t, err.Error(), "pull access denied")
}
