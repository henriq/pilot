package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"pilot/internal/ports"
)

// testDir creates a unique test directory within ~/.pilot/ and returns its path.
// The directory is automatically cleaned up when the test completes.
func testDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}
	dir := filepath.Join(home, ".pilot", "test-"+t.Name())
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck,gosec // cleanup
	return dir
}

func TestValidatePath_AllowsPathsWithinPilotDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}
	pilotPath := filepath.Join(home, ".pilot")

	tests := []struct {
		name string
		path string
	}{
		{"tilde path", "~/.pilot/config.yaml"},
		{"tilde nested path", "~/.pilot/contexts/dev/config.yaml"},
		{"absolute path", filepath.Join(pilotPath, "config.yaml")},
		{"absolute nested path", filepath.Join(pilotPath, "contexts", "dev", "config.yaml")},
		{"pilot directory itself", "~/.pilot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePath(tt.path)
			if err != nil {
				t.Errorf("validatePath(%q) returned error: %v, expected nil", tt.path, err)
			}
		})
	}
}

func TestValidatePath_AllowsConfigFile(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"tilde config path", "~/.pilot-config.yaml"},
		{"absolute config path", filepath.Join(home, ".pilot-config.yaml")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePath(tt.path)
			if err != nil {
				t.Errorf("validatePath(%q) returned error: %v, expected nil", tt.path, err)
			}
		})
	}
}

func TestValidatePath_DeniesPathsOutsidePilotDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"home directory", "~/"},
		{"other home subdirectory", "~/.config/something"},
		{"absolute root", "/etc/passwd"},
		{"absolute home", home},
		{"path traversal from pilot", "~/.pilot/../.ssh/id_rsa"},
		{"path traversal nested", "~/.pilot/foo/../../.ssh/id_rsa"},
		{"similar prefix", "~/.pilot-other/config"},
		{"similar prefix absolute", filepath.Join(home, ".pilot-other", "config")},
		{"config file with extra path", "~/.pilot-config.yaml/something"},
		{"config file traversal", "~/.pilot-config.yaml/../.ssh/id_rsa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePath(tt.path)
			if !errors.Is(err, ErrAccessDenied) {
				t.Errorf("validatePath(%q) = %v, expected ErrAccessDenied", tt.path, err)
			}
		})
	}
}

func TestValidatePath_DeniesEmptyPath(t *testing.T) {
	_, err := validatePath("")
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("validatePath(\"\") = %v, expected ErrAccessDenied", err)
	}
}

func TestValidatePath_DeniesSymlinkEscape(t *testing.T) {
	dir := testDir(t)

	// Create a symlink inside ~/.pilot/ that points outside
	symlinkPath := filepath.Join(dir, "escape-link")
	targetPath := "/tmp"

	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Skipf("cannot create symlink (possibly no permission): %v", err)
	}

	// Try to access a file through the symlink
	attackPath := filepath.Join(symlinkPath, "secret.txt")

	_, err := validatePath(attackPath)
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("validatePath(%q) = %v, expected ErrAccessDenied (symlink escape)", attackPath, err)
	}
}

func TestValidatePath_AllowsSymlinkWithinPilot(t *testing.T) {
	dir := testDir(t)

	// Create a subdirectory and a symlink to it within ~/.pilot/
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	symlinkPath := filepath.Join(dir, "link-to-subdir")
	if err := os.Symlink(subdir, symlinkPath); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Accessing through symlink within ~/.pilot/ should be allowed
	accessPath := filepath.Join(symlinkPath, "file.txt")

	_, err := validatePath(accessPath)
	if err != nil {
		t.Errorf("validatePath(%q) = %v, expected nil (symlink within ~/.pilot/)", accessPath, err)
	}
}

func TestOsFileSystem_AllMethods_DenyAccessOutsidePilot(t *testing.T) {
	fs := NewOsFileSystem()

	tests := []struct {
		name   string
		action func() error
	}{
		{"ReadFile", func() error { _, err := fs.ReadFile("/etc/passwd"); return err }},
		{"WriteFile", func() error { return fs.WriteFile("/tmp/test-file", []byte("x"), ports.ReadWrite) }},
		{"FileExists", func() error { _, err := fs.FileExists("/etc/passwd"); return err }},
		{"EnsureDirExists", func() error { return fs.EnsureDirExists("/tmp/test-dir/file") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			if !errors.Is(err, ErrAccessDenied) {
				t.Errorf("%s should return ErrAccessDenied, got: %v", tt.name, err)
			}
		})
	}
}

func TestOsFileSystem_AllMethods_DenyEmptyPath(t *testing.T) {
	fs := NewOsFileSystem()

	tests := []struct {
		name   string
		action func() error
	}{
		{"ReadFile", func() error { _, err := fs.ReadFile(""); return err }},
		{"WriteFile", func() error { return fs.WriteFile("", []byte("x"), ports.ReadWrite) }},
		{"FileExists", func() error { _, err := fs.FileExists(""); return err }},
		{"EnsureDirExists", func() error { return fs.EnsureDirExists("") }},
		{"MkdirAll", func() error { return fs.MkdirAll("", ports.ReadWriteExecute) }},
		{"RemoveAll", func() error { return fs.RemoveAll("") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			if !errors.Is(err, ErrAccessDenied) {
				t.Errorf("%s with empty path should return ErrAccessDenied, got: %v", tt.name, err)
			}
		})
	}
}

func TestOsFileSystem_ReadWriteRoundTrip_WithinPilot(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	testFile := filepath.Join(dir, "roundtrip.txt")
	content := []byte("test content")

	// Write
	err := fs.WriteFile(testFile, content, ports.ReadWrite)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Check exists
	exists, err := fs.FileExists(testFile)
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if !exists {
		t.Fatal("FileExists returned false, expected true")
	}

	// Read
	readContent, err := fs.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("ReadFile returned %q, expected %q", string(readContent), string(content))
	}
}

func TestOsFileSystem_ReadWriteConfigFile(t *testing.T) {
	fs := NewOsFileSystem()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	configFile := filepath.Join(home, ".pilot-config.yaml")

	// Check if config file exists and back it up
	existingContent, existsErr := os.ReadFile(configFile) //nolint:gosec // test reading known config path
	configExists := existsErr == nil

	if configExists {
		t.Cleanup(func() {
			os.WriteFile(configFile, existingContent, 0600) //nolint:errcheck,gosec // cleanup
		})
	} else {
		t.Cleanup(func() {
			os.Remove(configFile) //nolint:errcheck,gosec // cleanup
		})
	}

	// Write test content
	testContent := []byte("test: config\n")
	err = fs.WriteFile(configFile, testContent, ports.ReadWrite)
	if err != nil {
		t.Fatalf("WriteFile to config failed: %v", err)
	}

	// Read it back
	readContent, err := fs.ReadFile(configFile)
	if err != nil {
		t.Fatalf("ReadFile config failed: %v", err)
	}

	if string(readContent) != string(testContent) {
		t.Errorf("ReadFile returned %q, expected %q", string(readContent), string(testContent))
	}
}

func TestOsFileSystem_FileExists_ReturnsFalseForNonExistent(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	nonExistentFile := filepath.Join(dir, "does-not-exist.txt")

	exists, err := fs.FileExists(nonExistentFile)
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if exists {
		t.Error("FileExists returned true for non-existent file")
	}
}

func TestOsFileSystem_EnsureDirExists_CreatesParentDirectories(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	deepPath := filepath.Join(dir, "a", "b", "c", "file.txt")

	err := fs.EnsureDirExists(deepPath)
	if err != nil {
		t.Fatalf("EnsureDirExists failed: %v", err)
	}

	// Check that parent directory was created
	parentDir := filepath.Dir(deepPath)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("parent path is not a directory")
	}
}

func TestOsFileSystem_WriteFile_AccessModes(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	tests := []struct {
		name         string
		mode         ports.AccessMode
		expectedPerm os.FileMode
	}{
		{"ReadWrite", ports.ReadWrite, 0600},
		{"ReadWriteExecute", ports.ReadWriteExecute, 0700},
		{"ReadAllWriteOwner", ports.ReadAllWriteOwner, 0644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(dir, "mode-test-"+tt.name+".txt")

			err := fs.WriteFile(testFile, []byte("test"), tt.mode)
			if err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}

			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Stat failed: %v", err)
			}

			// Check permissions (masking to get only permission bits)
			actualPerm := info.Mode().Perm()
			if actualPerm != tt.expectedPerm {
				t.Errorf("file permissions = %o, expected %o", actualPerm, tt.expectedPerm)
			}
		})
	}
}

func TestOsFileSystem_DeniesSymlinkEscapeAttack(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	// Create a symlink inside ~/.pilot/ that points to /tmp
	symlinkPath := filepath.Join(dir, "malicious-link")
	if err := os.Symlink("/tmp", symlinkPath); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Try various operations through the symlink
	attackFile := filepath.Join(symlinkPath, "attack-test.txt")

	t.Run("ReadFile", func(t *testing.T) {
		_, err := fs.ReadFile(attackFile)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("ReadFile through symlink escape = %v, expected ErrAccessDenied", err)
		}
	})

	t.Run("WriteFile", func(t *testing.T) {
		err := fs.WriteFile(attackFile, []byte("pwned"), ports.ReadWrite)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("WriteFile through symlink escape = %v, expected ErrAccessDenied", err)
		}
	})

	t.Run("FileExists", func(t *testing.T) {
		_, err := fs.FileExists(attackFile)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("FileExists through symlink escape = %v, expected ErrAccessDenied", err)
		}
	})

	t.Run("EnsureDirExists", func(t *testing.T) {
		err := fs.EnsureDirExists(attackFile)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("EnsureDirExists through symlink escape = %v, expected ErrAccessDenied", err)
		}
	})

	t.Run("MkdirAll", func(t *testing.T) {
		err := fs.MkdirAll(attackFile, ports.ReadWriteExecute)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("MkdirAll through symlink escape = %v, expected ErrAccessDenied", err)
		}
	})

	t.Run("RemoveAll", func(t *testing.T) {
		err := fs.RemoveAll(attackFile)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("RemoveAll through symlink escape = %v, expected ErrAccessDenied", err)
		}
	})
}

func TestOsFileSystem_MkdirAll_CreatesDirectory(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	newDir := filepath.Join(dir, "new", "nested", "directory")

	err := fs.MkdirAll(newDir, ports.ReadWriteExecute)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestOsFileSystem_MkdirAll_DeniesAccessOutsidePilot(t *testing.T) {
	fs := NewOsFileSystem()

	err := fs.MkdirAll("/tmp/test-mkdir", ports.ReadWriteExecute)
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("MkdirAll outside ~/.pilot/ = %v, expected ErrAccessDenied", err)
	}
}

func TestOsFileSystem_RemoveAll_RemovesDirectory(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	// Create a directory with content
	subdir := filepath.Join(dir, "to-remove")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("test"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err := fs.RemoveAll(subdir)
	if err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	if _, err := os.Stat(subdir); !os.IsNotExist(err) {
		t.Error("directory should have been removed")
	}
}

func TestOsFileSystem_RemoveAll_DeniesAccessOutsidePilot(t *testing.T) {
	fs := NewOsFileSystem()

	err := fs.RemoveAll("/tmp/test-remove")
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("RemoveAll outside ~/.pilot/ = %v, expected ErrAccessDenied", err)
	}
}

func TestExpandPath_CrossPlatform(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde with forward slash", "~/.pilot/config", filepath.Join(home, ".pilot", "config")},
		{"tilde with backslash", "~\\.pilot\\config", filepath.Join(home, ".pilot", "config")},
		{"tilde only", "~", home},
		{"tilde with mixed separators", "~/.pilot\\subdir/file", filepath.Join(home, ".pilot", "subdir", "file")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.input)
			if err != nil {
				t.Fatalf("expandPath(%q) error: %v", tt.input, err)
			}
			// Compare cleaned paths to handle OS-specific separators
			if filepath.Clean(result) != filepath.Clean(tt.expected) {
				t.Errorf("expandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizePathSeparators(t *testing.T) {
	sep := string(filepath.Separator)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"forward slashes", "foo/bar/baz", "foo" + sep + "bar" + sep + "baz"},
		{"backslashes", "foo\\bar\\baz", "foo" + sep + "bar" + sep + "baz"},
		{"mixed separators", "foo/bar\\baz", "foo" + sep + "bar" + sep + "baz"},
		{"multiple consecutive forward slashes", "foo//bar", "foo" + sep + sep + "bar"},
		{"multiple consecutive backslashes", "foo\\\\bar", "foo" + sep + sep + "bar"},
		{"empty string", "", ""},
		{"single segment", "foo", "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePathSeparators(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathSeparators(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPathsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"identical paths", "/home/user/.pilot", "/home/user/.pilot", true},
		{"different paths", "/home/user/.pilot", "/home/other/.pilot", false},
		{"empty paths", "", "", true},
		{"one empty", "/home/user/.pilot", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("pathsEqual(%q, %q) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestPathsEqual_CaseSensitivity(t *testing.T) {
	// This test verifies behavior differs by OS
	// On Windows: case-insensitive, on Unix: case-sensitive
	a := "/Home/User/.pilot"
	b := "/home/user/.pilot"

	result := pathsEqual(a, b)

	// We can't assert the exact result since it depends on OS,
	// but we verify the function doesn't panic and returns a boolean
	t.Logf("pathsEqual(%q, %q) = %v (OS-dependent behavior)", a, b, result)
}

func TestPathHasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		prefix   string
		expected bool
	}{
		{"has prefix", "/home/user/.pilot/config", "/home/user/.pilot/", true},
		{"exact match", "/home/user/.pilot/", "/home/user/.pilot/", true},
		{"no prefix", "/home/user/.pilot/config", "/home/other/", false},
		{"partial match not prefix", "/home/user/.pilot-other", "/home/user/.pilot/", false},
		{"empty prefix", "/home/user/.pilot", "", true},
		{"empty path", "", "/home/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathHasPrefix(tt.path, tt.prefix)
			if result != tt.expected {
				t.Errorf("pathHasPrefix(%q, %q) = %v, expected %v", tt.path, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestPathHasPrefix_CaseSensitivity(t *testing.T) {
	// This test verifies behavior differs by OS
	// On Windows: case-insensitive, on Unix: case-sensitive
	path := "/Home/User/.pilot/config"
	prefix := "/home/user/.pilot/"

	result := pathHasPrefix(path, prefix)

	// We can't assert the exact result since it depends on OS,
	// but we verify the function doesn't panic and returns a boolean
	t.Logf("pathHasPrefix(%q, %q) = %v (OS-dependent behavior)", path, prefix, result)
}

func TestValidatePath_DeniesChainedSymlinkEscape(t *testing.T) {
	dir := testDir(t)

	// Create a chain of symlinks: link1 -> link2 -> /tmp
	link2Path := filepath.Join(dir, "link2")
	link1Path := filepath.Join(dir, "link1")

	// link2 points to /tmp (outside allowed paths)
	if err := os.Symlink("/tmp", link2Path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// link1 points to link2
	if err := os.Symlink(link2Path, link1Path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Try to access a file through the symlink chain
	attackPath := filepath.Join(link1Path, "secret.txt")

	_, err := validatePath(attackPath)
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("validatePath(%q) = %v, expected ErrAccessDenied (chained symlink escape)", attackPath, err)
	}
}

func TestOsFileSystem_DeniesChainedSymlinkEscape(t *testing.T) {
	fs := NewOsFileSystem()
	dir := testDir(t)

	// Create a chain of symlinks: link1 -> link2 -> link3 -> /tmp
	link3Path := filepath.Join(dir, "link3")
	link2Path := filepath.Join(dir, "link2")
	link1Path := filepath.Join(dir, "link1")

	// link3 points to /tmp (outside allowed paths)
	if err := os.Symlink("/tmp", link3Path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// link2 points to link3
	if err := os.Symlink(link3Path, link2Path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// link1 points to link2
	if err := os.Symlink(link2Path, link1Path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Try various operations through the symlink chain
	attackFile := filepath.Join(link1Path, "attack-test.txt")

	t.Run("WriteFile", func(t *testing.T) {
		err := fs.WriteFile(attackFile, []byte("pwned"), ports.ReadWrite)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("WriteFile through chained symlink escape = %v, expected ErrAccessDenied", err)
		}
	})

	t.Run("ReadFile", func(t *testing.T) {
		_, err := fs.ReadFile(attackFile)
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("ReadFile through chained symlink escape = %v, expected ErrAccessDenied", err)
		}
	})
}
