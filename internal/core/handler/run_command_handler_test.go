package handler

import (
	"errors"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRunCommandHandler_Handle_Success(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name:     "test-context",
		Services: []domain.Service{},
	}
	secrets := []*domain.Secret{}

	// For CreateTemplatingValues
	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	// For script execution (use current OS shell)
	shell, shellArg := getShellCommand()
	templater.On("Render", "echo hello", "test-script", mock.Anything).Return("echo hello", nil)
	commandRunner.On("RunInteractive", shell, []string{shellArg, "echo hello"}).Return(nil)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"test-script": "echo hello"}
	executionPlan := []string{"test-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
	templater.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_WithServiceDependency(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		Services: []domain.Service{
			{
				Name:        "my-service",
				GitRepoPath: "github.com/org/repo",
				GitRef:      "main",
				Path:        "/path/to/service",
			},
		},
	}
	secrets := []*domain.Secret{}

	// For CreateTemplatingValues
	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	// For script execution with service dependency (use current OS shell)
	shell, shellArg := getShellCommand()
	scriptWithDep := `cd {{.Services."my-service".path}} && make build`
	scm.On("Download", "github.com/org/repo", "main", "/path/to/service").Return(nil)
	templater.On("Render", scriptWithDep, "build-script", mock.Anything).Return("cd /path/to/service && make build", nil)
	commandRunner.On("RunInteractive", shell, []string{shellArg, "cd /path/to/service && make build"}).Return(nil)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"build-script": scriptWithDep}
	executionPlan := []string{"build-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
	templater.AssertExpectations(t)
	scm.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_LoadContextNameError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	expectedErr := errors.New("load context name error")
	configRepository.On("LoadCurrentContextName").Return("", expectedErr)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"test-script": "echo hello"}
	executionPlan := []string{"test-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_LoadConfigContextError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{Name: "test-context"}
	secrets := []*domain.Secret{}
	expectedErr := errors.New("load config context error")

	// First call (inside CreateTemplatingValues) succeeds
	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil).Once()
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	// Second call (direct in Handle) fails
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, expectedErr).Once()

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"test-script": "echo hello"}
	executionPlan := []string{"test-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_LoadSecretsError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{Name: "test-context"}
	expectedErr := errors.New("load secrets error")

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(nil, expectedErr)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"test-script": "echo hello"}
	executionPlan := []string{"test-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_ServiceNotFoundError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name:     "test-context",
		Services: []domain.Service{}, // No services configured
	}
	secrets := []*domain.Secret{}

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	// Script references a non-existent service
	scriptWithMissingService := `cd {{.Services."missing-service".path}} && make build`
	scripts := map[string]string{"build-script": scriptWithMissingService}
	executionPlan := []string{"build-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service 'missing-service' not found")
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_ServiceMissingGitInfoError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		Services: []domain.Service{
			{
				Name:        "my-service",
				GitRepoPath: "", // Missing git repo path
				GitRef:      "",
				Path:        "/path/to/service",
			},
		},
	}
	secrets := []*domain.Secret{}

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scriptWithDep := `cd {{.Services."my-service".path}} && make build`
	scripts := map[string]string{"build-script": scriptWithDep}
	executionPlan := []string{"build-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git repository path or ref is empty for service 'my-service'")
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_ScmDownloadError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name: "test-context",
		Services: []domain.Service{
			{
				Name:        "my-service",
				GitRepoPath: "github.com/org/repo",
				GitRef:      "main",
				Path:        "/path/to/service",
			},
		},
	}
	secrets := []*domain.Secret{}
	downloadErr := errors.New("download error")

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)
	scm.On("Download", "github.com/org/repo", "main", "/path/to/service").Return(downloadErr)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scriptWithDep := `cd {{.Services."my-service".path}} && make build`
	scripts := map[string]string{"build-script": scriptWithDep}
	executionPlan := []string{"build-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Equal(t, downloadErr, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_TemplateRenderError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name:     "test-context",
		Services: []domain.Service{},
	}
	secrets := []*domain.Secret{}
	renderErr := errors.New("template render error")

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)
	templater.On("Render", "echo hello", "test-script", mock.Anything).Return("", renderErr)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"test-script": "echo hello"}
	executionPlan := []string{"test-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Equal(t, renderErr, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
	templater.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_CommandRunError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name:     "test-context",
		Services: []domain.Service{},
	}
	secrets := []*domain.Secret{}
	runErr := errors.New("command execution error")

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	// Use current OS shell
	shell, shellArg := getShellCommand()
	templater.On("Render", "exit 1", "failing-script", mock.Anything).Return("exit 1", nil)
	commandRunner.On("RunInteractive", shell, []string{shellArg, "exit 1"}).Return(runErr)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{"failing-script": "exit 1"}
	executionPlan := []string{"failing-script"}

	err := sut.Handle(scripts, executionPlan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "script 'failing-script' failed")
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
	templater.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
}

func TestRunCommandHandler_Handle_MultipleScripts(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	secretsRepository := new(testutil.MockSecretsRepository)
	templater := new(testutil.MockTemplater)
	scm := new(testutil.MockScm)
	commandRunner := new(testutil.MockCommandRunner)

	configContext := &domain.ConfigurationContext{
		Name:     "test-context",
		Services: []domain.Service{},
	}
	secrets := []*domain.Secret{}

	configRepository.On("LoadCurrentContextName").Return("test-context", nil)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	secretsRepository.On("LoadSecrets", "test-context").Return(secrets, nil)

	// Use current OS shell
	shell, shellArg := getShellCommand()
	templater.On("Render", "echo first", "script1", mock.Anything).Return("echo first", nil)
	templater.On("Render", "echo second", "script2", mock.Anything).Return("echo second", nil)
	commandRunner.On("RunInteractive", shell, []string{shellArg, "echo first"}).Return(nil)
	commandRunner.On("RunInteractive", shell, []string{shellArg, "echo second"}).Return(nil)

	sut := NewRunCommandHandler(configRepository, secretsRepository, templater, scm, commandRunner)

	scripts := map[string]string{
		"script1": "echo first",
		"script2": "echo second",
	}
	executionPlan := []string{"script1", "script2"}

	err := sut.Handle(scripts, executionPlan)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	secretsRepository.AssertExpectations(t)
	templater.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
	commandRunner.AssertNumberOfCalls(t, "RunInteractive", 2)
}

func TestGetShellCommand_ReturnsBash(t *testing.T) {
	shell, shellArg := getShellCommand()

	assert.Equal(t, "bash", shell)
	assert.Equal(t, "-c", shellArg)
}
