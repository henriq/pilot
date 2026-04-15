package handler

import (
	"errors"
	"path/filepath"
	"testing"

	"pilot/internal/core/domain"
	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newCacheHandlerMocks() (*testutil.MockConfigRepository, *testutil.MockFileSystem, *testutil.MockTerminalInput) {
	return new(testutil.MockConfigRepository), new(testutil.MockFileSystem), new(testutil.MockTerminalInput)
}

func TestCacheCommandHandler_HandleStatus_NoCacheData(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", filepath.Join("~", ".pilot", "my-context")).Return(false, nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.NoError(t, err)
	configRepo.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestCacheCommandHandler_HandleStatus_LoadContextError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	expectedErr := errors.New("context error")
	configRepo.On("LoadCurrentContextName").Return("", expectedErr)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestCacheCommandHandler_HandleStatus_WithCacheEntries(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{
		"charts",
		"my-service",
		"ca",
	}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024*1024), nil)
	fs.On("DirSize", filepath.Join(contextPath, "my-service")).Return(int64(2048*1024), nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.NoError(t, err)
	fs.AssertNotCalled(t, "DirSize", filepath.Join(contextPath, "ca"))
	configRepo.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestCacheCommandHandler_HandleStatus_OnlyCacheDirectories(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{
		"ca",
	}, nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.NoError(t, err)
	fs.AssertNotCalled(t, "DirSize", mock.Anything)
}

func TestCacheCommandHandler_HandleStatus_AllContexts(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	config := &domain.Config{
		Contexts: []domain.ConfigurationContext{
			{Name: "context-a"},
			{Name: "context-b"},
		},
	}
	configRepo.On("LoadConfig").Return(config, nil)

	pathA := filepath.Join("~", ".pilot", "context-a")
	pathB := filepath.Join("~", ".pilot", "context-b")

	fs.On("FileExists", pathA).Return(true, nil)
	fs.On("FileExists", pathB).Return(true, nil)
	fs.On("ReadSubdirectories", pathA).Return([]string{"charts"}, nil)
	fs.On("ReadSubdirectories", pathB).Return([]string{"my-service"}, nil)
	fs.On("DirSize", filepath.Join(pathA, "charts")).Return(int64(1024), nil)
	fs.On("DirSize", filepath.Join(pathB, "my-service")).Return(int64(2048), nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(true)

	assert.NoError(t, err)
	configRepo.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestCacheCommandHandler_HandleStatus_AllContexts_SingleContext(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	config := &domain.Config{
		Contexts: []domain.ConfigurationContext{
			{Name: "context-a"},
		},
	}
	configRepo.On("LoadConfig").Return(config, nil)

	pathA := filepath.Join("~", ".pilot", "context-a")
	fs.On("FileExists", pathA).Return(true, nil)
	fs.On("ReadSubdirectories", pathA).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(pathA, "charts")).Return(int64(1024), nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(true)

	assert.NoError(t, err)
	configRepo.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestCacheCommandHandler_HandleStatus_AllContexts_NoCacheData(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	config := &domain.Config{
		Contexts: []domain.ConfigurationContext{
			{Name: "context-a"},
		},
	}
	configRepo.On("LoadConfig").Return(config, nil)
	fs.On("FileExists", filepath.Join("~", ".pilot", "context-a")).Return(false, nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(true)

	assert.NoError(t, err)
}

func TestCacheCommandHandler_HandleClear_Confirmed(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{
		"charts",
		"wrapper-charts",
		"ca",
	}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	fs.On("DirSize", filepath.Join(contextPath, "wrapper-charts")).Return(int64(2048), nil)
	termInput.On("IsTerminal").Return(true)
	termInput.On("ReadLine", "Continue? [y/N] ").Return("y", nil)
	fs.On("RemoveAll", filepath.Join(contextPath, "charts")).Return(nil)
	fs.On("RemoveAll", filepath.Join(contextPath, "wrapper-charts")).Return(nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(false, false)

	assert.NoError(t, err)
	fs.AssertCalled(t, "RemoveAll", filepath.Join(contextPath, "charts"))
	fs.AssertCalled(t, "RemoveAll", filepath.Join(contextPath, "wrapper-charts"))
	fs.AssertNotCalled(t, "RemoveAll", filepath.Join(contextPath, "ca"))
}

func TestCacheCommandHandler_HandleClear_Declined(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	termInput.On("IsTerminal").Return(true)
	termInput.On("ReadLine", "Continue? [y/N] ").Return("n", nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(false, false)

	assert.NoError(t, err)
	fs.AssertNotCalled(t, "RemoveAll", mock.Anything)
}

func TestCacheCommandHandler_HandleClear_SkipConfirmation(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	fs.On("RemoveAll", filepath.Join(contextPath, "charts")).Return(nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(true, false)

	assert.NoError(t, err)
	fs.AssertCalled(t, "RemoveAll", filepath.Join(contextPath, "charts"))
	termInput.AssertNotCalled(t, "IsTerminal")
	termInput.AssertNotCalled(t, "ReadLine", mock.Anything)
}

func TestCacheCommandHandler_HandleClear_NothingToClear(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(false, nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(true, false)

	assert.NoError(t, err)
	fs.AssertNotCalled(t, "RemoveAll", mock.Anything)
}

func TestCacheCommandHandler_HandleClear_NonInteractiveWithoutYes(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	termInput.On("IsTerminal").Return(false)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(false, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Use --yes")
	fs.AssertNotCalled(t, "RemoveAll", mock.Anything)
}

func TestCacheCommandHandler_HandleClear_AllContexts_Confirmed(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	config := &domain.Config{
		Contexts: []domain.ConfigurationContext{
			{Name: "context-a"},
			{Name: "context-b"},
		},
	}
	configRepo.On("LoadConfig").Return(config, nil)

	pathA := filepath.Join("~", ".pilot", "context-a")
	pathB := filepath.Join("~", ".pilot", "context-b")

	fs.On("FileExists", pathA).Return(true, nil)
	fs.On("FileExists", pathB).Return(true, nil)
	fs.On("ReadSubdirectories", pathA).Return([]string{"charts"}, nil)
	fs.On("ReadSubdirectories", pathB).Return([]string{"my-service"}, nil)
	fs.On("DirSize", filepath.Join(pathA, "charts")).Return(int64(1024), nil)
	fs.On("DirSize", filepath.Join(pathB, "my-service")).Return(int64(2048), nil)
	termInput.On("IsTerminal").Return(true)
	termInput.On("ReadLine", "Continue? [y/N] ").Return("y", nil)
	fs.On("RemoveAll", filepath.Join(pathA, "charts")).Return(nil)
	fs.On("RemoveAll", filepath.Join(pathB, "my-service")).Return(nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(false, true)

	assert.NoError(t, err)
	fs.AssertCalled(t, "RemoveAll", filepath.Join(pathA, "charts"))
	fs.AssertCalled(t, "RemoveAll", filepath.Join(pathB, "my-service"))
	termInput.AssertCalled(t, "ReadLine", "Continue? [y/N] ")
}

func TestCacheCommandHandler_HandleClear_AllContexts(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	config := &domain.Config{
		Contexts: []domain.ConfigurationContext{
			{Name: "context-a"},
			{Name: "context-b"},
		},
	}
	configRepo.On("LoadConfig").Return(config, nil)

	pathA := filepath.Join("~", ".pilot", "context-a")
	pathB := filepath.Join("~", ".pilot", "context-b")

	fs.On("FileExists", pathA).Return(true, nil)
	fs.On("FileExists", pathB).Return(true, nil)
	fs.On("ReadSubdirectories", pathA).Return([]string{"charts"}, nil)
	fs.On("ReadSubdirectories", pathB).Return([]string{"my-service"}, nil)
	fs.On("DirSize", filepath.Join(pathA, "charts")).Return(int64(1024), nil)
	fs.On("DirSize", filepath.Join(pathB, "my-service")).Return(int64(2048), nil)
	fs.On("RemoveAll", filepath.Join(pathA, "charts")).Return(nil)
	fs.On("RemoveAll", filepath.Join(pathB, "my-service")).Return(nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(true, true)

	assert.NoError(t, err)
	fs.AssertCalled(t, "RemoveAll", filepath.Join(pathA, "charts"))
	fs.AssertCalled(t, "RemoveAll", filepath.Join(pathB, "my-service"))
}

func TestCacheCommandHandler_HandleStatus_SkipsEmptyDirectories(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts", "empty-dir"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	fs.On("DirSize", filepath.Join(contextPath, "empty-dir")).Return(int64(0), nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.NoError(t, err)
	configRepo.AssertExpectations(t)
	fs.AssertExpectations(t)
}

func TestCacheCommandHandler_HandleStatus_AllEmptyDirectories(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"empty-dir"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "empty-dir")).Return(int64(0), nil)

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.NoError(t, err)
}

func TestCacheCommandHandler_HandleStatus_FileExistsError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", filepath.Join("~", ".pilot", "my-context")).Return(false, errors.New("permission denied"))

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.EqualError(t, err, "permission denied")
}

func TestCacheCommandHandler_HandleStatus_ReadDirError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return(nil, errors.New("read error"))

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.ErrorContains(t, err, "failed to read context directory")
}

func TestCacheCommandHandler_HandleStatus_DirSizeError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(0), errors.New("stat error"))

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(false)

	assert.ErrorContains(t, err, "failed to calculate size of")
}

func TestCacheCommandHandler_HandleStatus_AllContexts_LoadConfigError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()

	configRepo.On("LoadConfig").Return(nil, errors.New("config error"))

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleStatus(true)

	assert.EqualError(t, err, "config error")
}

func TestCacheCommandHandler_HandleClear_RemoveAllError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	fs.On("RemoveAll", filepath.Join(contextPath, "charts")).Return(errors.New("remove error"))

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(true, false)

	assert.ErrorContains(t, err, "failed to remove")
}

func TestCacheCommandHandler_HandleClear_ReadLineError(t *testing.T) {
	configRepo, fs, termInput := newCacheHandlerMocks()
	contextPath := filepath.Join("~", ".pilot", "my-context")

	configRepo.On("LoadCurrentContextName").Return("my-context", nil)
	fs.On("FileExists", contextPath).Return(true, nil)
	fs.On("ReadSubdirectories", contextPath).Return([]string{"charts"}, nil)
	fs.On("DirSize", filepath.Join(contextPath, "charts")).Return(int64(1024), nil)
	termInput.On("IsTerminal").Return(true)
	termInput.On("ReadLine", "Continue? [y/N] ").Return("", errors.New("io error"))

	sut := NewCacheCommandHandler(configRepo, fs, termInput)

	err := sut.HandleClear(false, false)

	assert.ErrorContains(t, err, "failed to read confirmation")
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatSize(tt.bytes))
		})
	}
}
