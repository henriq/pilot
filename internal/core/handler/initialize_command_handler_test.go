package handler

import (
	"pilot/internal/testutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitializeCommandHandler_HandleReturnsErrorIfConfigExists(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("ConfigExists").Return(true, nil)
	sut := InitializeCommandHandler{
		configRepository: configRepository,
	}

	result := sut.Handle()

	assert.NotNil(t, result)
}

func TestInitializeCommandHandler_HandleWritesDefaultConfigIfNoConfigExists(t *testing.T) {
	configRepository := new(testutil.MockConfigRepository)
	configRepository.On("ConfigExists").Return(false, nil)
	sut := NewInitializeCommandHandler(configRepository)
	configRepository.On("SaveConfig", mock.Anything).Return(nil)

	result := sut.Handle()

	assert.Nil(t, result)
	configRepository.AssertExpectations(t)
}
