package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"pilot/internal/ports"
)

// TestFileSystem provides real file system operations sandboxed within a temporary directory.
// All paths are automatically resolved relative to the sandbox directory.
// Use this in tests that need to actually read/write files.
// For unit tests that mock file system calls, use MockFileSystem instead.
type TestFileSystem struct {
	baseDir string
}

// NewTestFileSystem creates a sandboxed file system within a temporary directory.
// The directory is automatically cleaned up when the test completes.
func NewTestFileSystem(t *testing.T) *TestFileSystem {
	t.Helper()
	baseDir := t.TempDir()
	return &TestFileSystem{baseDir: baseDir}
}

// BaseDir returns the sandbox base directory path.
// Use this when you need to construct paths or verify file locations.
func (f *TestFileSystem) BaseDir() string {
	return f.baseDir
}

// resolvePath converts a path to be relative to the sandbox directory.
// Absolute paths are joined with the base directory.
// Relative paths are also joined with the base directory.
func (f *TestFileSystem) resolvePath(path string) string {
	// Strip leading slash to make it relative
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		// Remove the root to make it relative (e.g., "/foo/bar" -> "foo/bar")
		cleanPath = cleanPath[1:]
	}
	return filepath.Join(f.baseDir, cleanPath)
}

func (f *TestFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(f.resolvePath(path))
}

func (f *TestFileSystem) WriteFile(path string, content []byte, _ ports.AccessMode) error {
	resolved := f.resolvePath(path)
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(resolved), 0700); err != nil {
		return err
	}
	return os.WriteFile(resolved, content, 0600)
}

func (f *TestFileSystem) EnsureDirExists(path string) error {
	return os.MkdirAll(filepath.Dir(f.resolvePath(path)), 0700)
}

func (f *TestFileSystem) FileExists(path string) (bool, error) {
	_, err := os.Stat(f.resolvePath(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (f *TestFileSystem) MkdirAll(path string, _ ports.AccessMode) error {
	return os.MkdirAll(f.resolvePath(path), 0700)
}

func (f *TestFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(f.resolvePath(path))
}

func (f *TestFileSystem) HomeDir() (string, error) {
	// Return the sandbox base directory as a mock "home"
	return f.baseDir, nil
}
