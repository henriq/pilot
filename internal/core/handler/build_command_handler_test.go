package handler

import (
	"testing"

	"dx/internal/core/domain"
	"dx/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBuildCommandHandler_HandleBuildsAllServices(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo",
						GitRef:                   "any-branch",
					},
				},
				Profiles:     []string{"all"},
				RemoteImages: []string{"any-image"},
			},
			{
				Name: "service-2",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image-2",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo-2",
						GitRef:                   "any-branch-2",
					},
				},
				Profiles:     []string{"all"},
				RemoteImages: []string{"any-image-2"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	scm.On(
		"Download",
		configContext.Services[0].DockerImages[0].GitRepoPath,
		configContext.Services[0].DockerImages[0].GitRef,
		configContext.Services[0].DockerImages[0].Path,
	).Return(nil)
	containerImageRepository.On("PullImage", configContext.Services[0].RemoteImages[0]).Return(nil)
	containerImageRepository.On("BuildImage", configContext.Services[0].DockerImages[0]).Return(nil)
	scm.On(
		"Download",
		configContext.Services[1].DockerImages[0].GitRepoPath,
		configContext.Services[1].DockerImages[0].GitRef,
		configContext.Services[1].DockerImages[0].Path,
	).Return(nil)
	containerImageRepository.On("PullImage", configContext.Services[1].RemoteImages[0]).Return(nil)
	containerImageRepository.On("BuildImage", configContext.Services[1].DockerImages[0]).Return(nil)

	sut := BuildCommandHandler{
		configRepository:         configRepository,
		scm:                      scm,
		containerImageRepository: containerImageRepository,
	}

	result := sut.Handle([]string{}, "all")

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestBuildCommandHandler_HandleBuildsOnlySelectedService(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo",
						GitRef:                   "any-branch",
					},
				},
				Profiles:     []string{"all"},
				RemoteImages: []string{"any-image"},
			},
			{
				Name: "service-2",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image-2",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo-2",
						GitRef:                   "any-branch-2",
					},
				},
				Profiles:     []string{"all"},
				RemoteImages: []string{"any-image-2"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	scm.On(
		"Download",
		configContext.Services[0].DockerImages[0].GitRepoPath,
		configContext.Services[0].DockerImages[0].GitRef,
		configContext.Services[0].DockerImages[0].Path,
	).Return(nil)
	containerImageRepository.On("PullImage", configContext.Services[0].RemoteImages[0]).Return(nil)
	containerImageRepository.On("BuildImage", configContext.Services[0].DockerImages[0]).Return(nil)

	sut := ProvideBuildCommandHandler(
		configRepository,
		scm,
		containerImageRepository,
	)

	result := sut.Handle([]string{configContext.Services[0].Name}, "default")

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestBuildCommandHandler_HandleBuildsOnlyServicesInSelectedProfile(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo",
						GitRef:                   "any-branch",
					},
				},
				Profiles:     []string{"selected"},
				RemoteImages: []string{"any-image"},
			},
			{
				Name: "service-2",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image-2",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo-2",
						GitRef:                   "any-branch-2",
					},
				},
				Profiles:     []string{"not-selected"},
				RemoteImages: []string{"any-image-2"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	scm.On(
		"Download",
		configContext.Services[0].DockerImages[0].GitRepoPath,
		configContext.Services[0].DockerImages[0].GitRef,
		configContext.Services[0].DockerImages[0].Path,
	).Return(nil)
	containerImageRepository.On("PullImage", configContext.Services[0].RemoteImages[0]).Return(nil)
	containerImageRepository.On("BuildImage", configContext.Services[0].DockerImages[0]).Return(nil)

	sut := ProvideBuildCommandHandler(
		configRepository,
		scm,
		containerImageRepository,
	)

	result := sut.Handle([]string{}, "selected")

	assert.Nil(t, result)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}

func TestBuildCommandHandler_Handle_LoadConfigError(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(nil, assert.AnError)
	scm := new(testutil.MockScm)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	sut := ProvideBuildCommandHandler(
		configRepository,
		scm,
		containerImageRepository,
	)

	result := sut.Handle([]string{}, "all")

	assert.ErrorIs(t, result, assert.AnError)
	scm.AssertNotCalled(t, "Download", mock.Anything, mock.Anything, mock.Anything)
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything)
}

func TestBuildCommandHandler_Handle_DownloadError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo",
						GitRef:                   "any-branch",
					},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].DockerImages[0].GitRepoPath,
		configContext.Services[0].DockerImages[0].GitRef,
		configContext.Services[0].DockerImages[0].Path,
	).Return(assert.AnError)
	containerImageRepository := new(testutil.MockContainerImageRepository)

	sut := ProvideBuildCommandHandler(
		configRepository,
		scm,
		containerImageRepository,
	)

	result := sut.Handle([]string{}, "all")

	assert.ErrorIs(t, result, assert.AnError)
	containerImageRepository.AssertNotCalled(t, "BuildImage", mock.Anything)
}

func TestBuildCommandHandler_Handle_BuildImageError(t *testing.T) {
	configContext := &domain.ConfigurationContext{
		Services: []domain.Service{
			{
				Name: "service-1",
				DockerImages: []domain.DockerImage{
					{
						Name:                     "any-image",
						DockerfilePath:           ".",
						BuildContextRelativePath: "",
						GitRepoPath:              "any-repo",
						GitRef:                   "any-branch",
					},
				},
				Profiles: []string{"all"},
			},
		},
	}
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("LoadCurrentConfigurationContext").Return(configContext, nil)
	scm := new(testutil.MockScm)
	scm.On(
		"Download",
		configContext.Services[0].DockerImages[0].GitRepoPath,
		configContext.Services[0].DockerImages[0].GitRef,
		configContext.Services[0].DockerImages[0].Path,
	).Return(nil)
	containerImageRepository := new(testutil.MockContainerImageRepository)
	containerImageRepository.On("BuildImage", configContext.Services[0].DockerImages[0]).Return(assert.AnError)

	sut := ProvideBuildCommandHandler(
		configRepository,
		scm,
		containerImageRepository,
	)

	result := sut.Handle([]string{}, "all")

	assert.ErrorIs(t, result, assert.AnError)
	scm.AssertExpectations(t)
	containerImageRepository.AssertExpectations(t)
}
