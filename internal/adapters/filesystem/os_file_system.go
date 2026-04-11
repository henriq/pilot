package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pilot/internal/ports"
	"runtime"
	"strings"
)

var _ ports.FileSystem = (*OsFileSystem)(nil)

var ErrAccessDenied = errors.New("access denied: path must be within ~/.pilot/ or be ~/.pilot-config.yaml")

type OsFileSystem struct{}

func NewOsFileSystem() *OsFileSystem {
	return &OsFileSystem{}
}

// expandPath expands ~ to the user's home directory and returns the absolute path.
// Works cross-platform (Unix, macOS, Windows) by normalizing path separators.
func expandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		// Get the part after ~, stripping any leading separator (/ or \)
		rest := path[1:]
		if len(rest) > 0 && (rest[0] == '/' || rest[0] == '\\') {
			rest = rest[1:]
		}
		// Normalize path separators to the OS-native separator for cross-platform compatibility
		rest = normalizePathSeparators(rest)
		path = filepath.Join(home, rest)
	}
	return filepath.Abs(path)
}

// normalizePathSeparators converts all path separators to the OS-native separator.
// This allows paths copied from Windows (with \) to work on Unix and vice versa.
func normalizePathSeparators(path string) string {
	// Replace both forward and back slashes with the OS-native separator
	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	path = strings.ReplaceAll(path, "\\", string(filepath.Separator))
	return path
}

// allowedPaths returns the allowed path patterns: ~/.pilot/ directory and ~/.pilot-config.yaml file.
func allowedPaths() (pilotDir string, configFile string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".pilot"), filepath.Join(home, ".pilot-config.yaml"), nil
}

// resolveSymlinks resolves symlinks in a path, handling non-existent files by
// resolving the existing parent directory and appending the remaining path.
func resolveSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !os.IsNotExist(err) {
		return "", err
	}

	// File doesn't exist - resolve parent directory and append filename
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Parent also doesn't exist, keep resolving up the tree
			resolvedDir, err = resolveSymlinks(dir)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	return filepath.Join(resolvedDir, base), nil
}

// validatePath checks that the given path is within ~/.pilot/ or is ~/.pilot-config.yaml.
// It also resolves symlinks to prevent symlink escape attacks.
func validatePath(path string) (string, error) {
	if path == "" {
		return "", ErrAccessDenied
	}

	absPath, err := expandPath(path)
	if err != nil {
		return "", err
	}

	pilotDir, configFile, err := allowedPaths()
	if err != nil {
		return "", err
	}

	// Clean the path first
	cleanPath := filepath.Clean(absPath)

	// Resolve symlinks to prevent symlink escape attacks
	// If resolution fails (e.g., path traverses through a file), deny access
	resolvedPath, err := resolveSymlinks(cleanPath)
	if err != nil {
		return "", ErrAccessDenied
	}

	// Check if path is the config file (case-insensitive on Windows)
	if pathsEqual(resolvedPath, configFile) {
		return resolvedPath, nil
	}

	// Check if path is within ~/.pilot/ directory (case-insensitive on Windows)
	if pathsEqual(resolvedPath, pilotDir) || pathHasPrefix(resolvedPath, pilotDir+string(filepath.Separator)) {
		return resolvedPath, nil
	}

	return "", ErrAccessDenied
}

// pathsEqual compares two paths for equality.
// On Windows, comparison is case-insensitive since the filesystem is case-insensitive.
func pathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// pathHasPrefix checks if path starts with prefix.
// On Windows, comparison is case-insensitive since the filesystem is case-insensitive.
func pathHasPrefix(path, prefix string) bool {
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix))
	}
	return strings.HasPrefix(path, prefix)
}

func (f *OsFileSystem) ReadFile(path string) ([]byte, error) {
	validPath, err := validatePath(path)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(validPath) //nolint:gosec // path validated by validatePath()
}

func (f *OsFileSystem) WriteFile(path string, content []byte, accessMode ports.AccessMode) error {
	validPath, err := validatePath(path)
	if err != nil {
		return err
	}

	err = f.ensureDirExistsInternal(validPath)
	if err != nil {
		return fmt.Errorf("failed to ensure directory exists: %w", err)
	}

	if err := os.WriteFile(validPath, content, getOsFileModeForAccessMode(accessMode)); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func (f *OsFileSystem) EnsureDirExists(path string) error {
	validPath, err := validatePath(path)
	if err != nil {
		return err
	}

	return f.ensureDirExistsInternal(validPath)
}

func (f *OsFileSystem) ensureDirExistsInternal(validPath string) error {
	if err := os.MkdirAll(filepath.Dir(validPath), getOsFileModeForAccessMode(ports.ReadWriteExecute)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

func (f *OsFileSystem) FileExists(path string) (bool, error) {
	validPath, err := validatePath(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(validPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check if file exists: %w", err)
}

func (f *OsFileSystem) MkdirAll(path string, accessMode ports.AccessMode) error {
	validPath, err := validatePath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(validPath, getOsFileModeForAccessMode(accessMode)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

func (f *OsFileSystem) RemoveAll(path string) error {
	validPath, err := validatePath(path)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(validPath); err != nil {
		return fmt.Errorf("failed to remove path: %w", err)
	}
	return nil
}

func (f *OsFileSystem) HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return home, nil
}

// getOsFileModeForAccessMode converts AccessMode to os.FileMode.
// Note: On Windows, Unix permission bits are largely ignored by the OS.
// Go's os package handles this gracefully - files are created with default
// Windows permissions. For sensitive files (like secrets), Windows relies
// on NTFS ACLs inherited from the parent directory rather than these bits.
func getOsFileModeForAccessMode(accessMode ports.AccessMode) os.FileMode {
	switch accessMode {
	case ports.ReadWrite:
		return 0600
	case ports.ReadWriteExecute:
		return 0700
	case ports.ReadAllWriteOwner:
		return 0644
	default:
		return 0600
	}
}
