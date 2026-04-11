package testutil

import (
	"pilot/internal/ports"

	"github.com/stretchr/testify/mock"
)

// AnyAccessMode is a matcher that matches any ports.AccessMode value.
// Use this in mock expectations when the exact access mode doesn't matter.
var AnyAccessMode = mock.MatchedBy(func(mode ports.AccessMode) bool {
	return true
})

type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileSystem) WriteFile(path string, content []byte, accessMode ports.AccessMode) error {
	args := m.Called(path, content, accessMode)
	return args.Error(0)
}

func (m *MockFileSystem) EnsureDirExists(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockFileSystem) FileExists(path string) (bool, error) {
	args := m.Called(path)
	return args.Bool(0), args.Error(1)
}

func (m *MockFileSystem) MkdirAll(path string, accessMode ports.AccessMode) error {
	args := m.Called(path, accessMode)
	return args.Error(0)
}

func (m *MockFileSystem) RemoveAll(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockFileSystem) ReadSubdirectories(path string) ([]string, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockFileSystem) DirSize(path string) (int64, error) {
	args := m.Called(path)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockFileSystem) HomeDir() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
