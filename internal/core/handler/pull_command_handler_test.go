package handler

import (
	"errors"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPullCommandHandler_Handle_PullsAllImages(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "docker-image-1"},
				},
				RemoteImages: []string{"remote-image-1"},
				Profiles:     []string{"all"},
			},
			{
				Name: "service-2",
				DockerImages: []domain.DockerImage{
					{Name: "docker-image-2"},
				},
				RemoteImages: []string{"remote-image-2"},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "docker-image-1").Return(nil).Once()
	containerImageRepository.On("PullImage", "docker-image-2").Return(nil).Once()
	containerImageRepository.On("PullImage", "remote-image-1").Return(nil).Once()
	containerImageRepository.On("PullImage", "remote-image-2").Return(nil).Once()

	terminalInput := new(testutil.MockTerminalInput)
	terminalInput.On("IsTerminal").Return(true)
	terminalInput.On("ReadLine", "Continue? [y/N] ").Return("y", nil)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestPullCommandHandler_Handle_FiltersServicesByProfile(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"remote-image-1"},
				Profiles:     []string{"selected"},
			},
			{
				Name:         "service-2",
				RemoteImages: []string{"remote-image-2"},
				Profiles:     []string{"not-selected"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "remote-image-1").Return(nil)

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "selected", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", "remote-image-2")
}

func TestPullCommandHandler_Handle_FiltersServicesByName(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"remote-image-1"},
				Profiles:     []string{"all"},
			},
			{
				Name:         "service-2",
				RemoteImages: []string{"remote-image-2"},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "remote-image-1").Return(nil)

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{"service-1"}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", "remote-image-2")
}

func TestPullCommandHandler_Handle_DeduplicatesImages(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"shared-image", "unique-image-1"},
				Profiles:     []string{"all"},
			},
			{
				Name:         "service-2",
				RemoteImages: []string{"shared-image", "unique-image-2"},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "shared-image").Return(nil).Once()
	containerImageRepository.On("PullImage", "unique-image-1").Return(nil).Once()
	containerImageRepository.On("PullImage", "unique-image-2").Return(nil).Once()

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestPullCommandHandler_Handle_NoImagesReturnsEarly(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{},
				DockerImages: []domain.DockerImage{},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", mock.Anything)
}

func TestPullCommandHandler_Handle_PullError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"image-1"},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	expectedErr := errors.New("pull failed")
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "image-1").Return(expectedErr)

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
}

func TestPullCommandHandler_Handle_ConfirmationPrompt_UserConfirms(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "local-image"},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "local-image").Return(nil)

	terminalInput := new(testutil.MockTerminalInput)
	terminalInput.On("IsTerminal").Return(true)
	terminalInput.On("ReadLine", "Continue? [y/N] ").Return("yes", nil)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	terminalInput.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestPullCommandHandler_Handle_ConfirmationPrompt_UserDeclines(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "local-image"},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)

	terminalInput := new(testutil.MockTerminalInput)
	terminalInput.On("IsTerminal").Return(true)
	terminalInput.On("ReadLine", "Continue? [y/N] ").Return("n", nil)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	terminalInput.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", mock.Anything)
}

func TestPullCommandHandler_Handle_SkipConfirmation(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "local-image"},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "local-image").Return(nil)

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	// skipConfirmation = true
	err := sut.Handle([]string{}, "all", true)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
	terminalInput.AssertNotCalled(t, "IsTerminal")
	terminalInput.AssertNotCalled(t, "ReadLine", mock.Anything)
}

func TestPullCommandHandler_Handle_NonTerminal_RequiresYesFlag(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "local-image"},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)

	terminalInput := new(testutil.MockTerminalInput)
	terminalInput.On("IsTerminal").Return(false)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Use --yes to skip")
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", mock.Anything)
}

func TestPullCommandHandler_Handle_LoadConfigError(t *testing.T) {
	expectedErr := errors.New("config error")
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, expectedErr)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	configRepository.AssertExpectations(t)
}

func TestPullCommandHandler_Handle_ReadLineError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{Name: "local-image"},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)

	expectedErr := errors.New("input error")
	terminalInput := new(testutil.MockTerminalInput)
	terminalInput.On("IsTerminal").Return(true)
	terminalInput.On("ReadLine", "Continue? [y/N] ").Return("", expectedErr)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read confirmation")
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", mock.Anything)
}

func TestPullCommandHandler_Handle_ServiceNameBypassesProfileFilter(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"remote-image-1"},
				Profiles:     []string{"infra"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "remote-image-1").Return(nil).Once()

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{"service-1"}, "default", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestPullCommandHandler_Handle_NonexistentServiceReturnsEarly(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"remote-image-1"},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{"nonexistent"}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertNotCalled(t, "PullImage", mock.Anything)
}

func TestPullCommandHandler_Handle_RemoteOnlyImagesSkipConfirmation(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name:         "service-1",
				RemoteImages: []string{"remote-image-1"},
				Profiles:     []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)

	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("PullImage", "remote-image-1").Return(nil).Once()

	terminalInput := new(testutil.MockTerminalInput)

	sut := NewPullCommandHandler(configRepository, containerImageRepository, terminalInput)

	err := sut.Handle([]string{}, "all", false)

	assert.NoError(t, err)
	configRepository.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
	terminalInput.AssertNotCalled(t, "IsTerminal")
	terminalInput.AssertNotCalled(t, "ReadLine", mock.Anything)
}
