package arguments

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"pilot/internal/core/domain"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceNameColonCompletions_OffersServicesWithColonSuffix(t *testing.T) {
	services := []domain.Service{{Name: "api"}, {Name: "worker"}}

	completions := serviceNameColonCompletions(services)

	assert.Equal(t, []cobra.Completion{"api:", "worker:"}, completions)
}

func TestServiceNameColonCompletions_EmptyServicesYieldsEmptyCompletions(t *testing.T) {
	completions := serviceNameColonCompletions(nil)

	assert.Empty(t, completions)
}

func TestPrefixedDirectoryCompletions_ListsMatchingSubdirectoriesWithServicePrefix(t *testing.T) {
	rootDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "charts"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "chimera"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "other"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(rootDirectory, "chart.yaml"), []byte("x"), 0o600))

	completions := prefixedDirectoryCompletions("api:", rootDirectory+string(filepath.Separator)+"ch")

	separator := string(filepath.Separator)
	require.Len(t, completions, 2)
	assert.ElementsMatch(t, []cobra.Completion{
		"api:" + rootDirectory + separator + "charts" + separator,
		"api:" + rootDirectory + separator + "chimera" + separator,
	}, completions)
}

func TestPrefixedDirectoryCompletions_ListsCurrentDirectoryWhenNoPath(t *testing.T) {
	rootDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "charts"), 0o750))
	t.Chdir(rootDirectory)

	completions := prefixedDirectoryCompletions("api:", "")

	assert.Equal(t, []cobra.Completion{"api:charts" + string(filepath.Separator)}, completions)
}

func TestPrefixedDirectoryCompletions_ReturnsNilWhenDirectoryDoesNotExist(t *testing.T) {
	missingDirectory := filepath.Join(t.TempDir(), "does-not-exist") + string(filepath.Separator)

	completions := prefixedDirectoryCompletions("api:", missingDirectory)

	assert.Nil(t, completions)
}

func TestPrefixedDirectoryCompletions_ReturnsEmptyWhenNoEntriesMatchPrefix(t *testing.T) {
	rootDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "charts"), 0o750))

	completions := prefixedDirectoryCompletions("api:", rootDirectory+string(filepath.Separator)+"zz")

	assert.NotNil(t, completions)
	assert.Empty(t, completions)
}

func TestPrefixedDirectoryCompletions_FollowsSymlinksToDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Symlink requires elevated privileges on Windows")
	}
	rootDirectory := t.TempDir()
	realChartDirectory := filepath.Join(rootDirectory, "real-charts")
	require.NoError(t, os.Mkdir(realChartDirectory, 0o750))
	symlinkPath := filepath.Join(rootDirectory, "linked-charts")
	require.NoError(t, os.Symlink(realChartDirectory, symlinkPath))

	completions := prefixedDirectoryCompletions("api:", rootDirectory+string(filepath.Separator)+"linked")

	assert.Equal(t, []cobra.Completion{
		"api:" + rootDirectory + string(filepath.Separator) + "linked-charts" + string(filepath.Separator),
	}, completions)
}

func TestPrefixedDirectoryCompletions_HidesDotDirectoriesUnlessPrefixStartsWithDot(t *testing.T) {
	rootDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, ".cache"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "charts"), 0o750))

	completionsWithoutDot := prefixedDirectoryCompletions("api:", rootDirectory+string(filepath.Separator))
	assert.Equal(t, []cobra.Completion{
		"api:" + rootDirectory + string(filepath.Separator) + "charts" + string(filepath.Separator),
	}, completionsWithoutDot)

	completionsWithDot := prefixedDirectoryCompletions("api:", rootDirectory+string(filepath.Separator)+".")
	assert.Equal(t, []cobra.Completion{
		"api:" + rootDirectory + string(filepath.Separator) + ".cache" + string(filepath.Separator),
	}, completionsWithDot)
}

func TestPrefixedDirectoryCompletions_ExpandsLeadingTildeSlash(t *testing.T) {
	homeDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(homeDirectory, "charts"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(homeDirectory, "chimera"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(homeDirectory, "other"), 0o750))
	t.Setenv("HOME", homeDirectory)
	t.Setenv("USERPROFILE", homeDirectory)

	completions := prefixedDirectoryCompletions("api:", "~/ch")

	separator := string(filepath.Separator)
	assert.ElementsMatch(t, []cobra.Completion{
		"api:~/charts" + separator,
		"api:~/chimera" + separator,
	}, completions)
}

func TestPrefixedDirectoryCompletions_ExpandsBareTilde(t *testing.T) {
	homeDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(homeDirectory, "charts"), 0o750))
	t.Setenv("HOME", homeDirectory)
	t.Setenv("USERPROFILE", homeDirectory)

	completions := prefixedDirectoryCompletions("api:", "~")

	separator := string(filepath.Separator)
	assert.Equal(t, []cobra.Completion{
		"api:~" + separator + "charts" + separator,
	}, completions)
}

func TestPrefixedDirectoryCompletions_LeavesNonLeadingTildeLiteral(t *testing.T) {
	rootDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "~tilde"), 0o750))

	completions := prefixedDirectoryCompletions("api:", rootDirectory+string(filepath.Separator)+"~")

	separator := string(filepath.Separator)
	assert.Equal(t, []cobra.Completion{
		"api:" + rootDirectory + separator + "~tilde" + separator,
	}, completions)
}

func TestAnchorBareVolumeToRoot(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		volumeName    string
		pathSeparator string
		expected      string
	}{
		{
			name:          "windows bare drive letter is anchored to drive root",
			path:          "C:",
			volumeName:    "C:",
			pathSeparator: `\`,
			expected:      `C:\`,
		},
		{
			name:          "windows UNC volume is anchored to share root",
			path:          `\\server\share`,
			volumeName:    `\\server\share`,
			pathSeparator: `\`,
			expected:      `\\server\share\`,
		},
		{
			name:          "windows drive-rooted path is left unchanged",
			path:          `C:\charts`,
			volumeName:    "C:",
			pathSeparator: `\`,
			expected:      `C:\charts`,
		},
		{
			name:          "unix path is left unchanged",
			path:          "/charts",
			volumeName:    "",
			pathSeparator: "/",
			expected:      "/charts",
		},
		{
			name:          "empty path is left unchanged",
			path:          "",
			volumeName:    "",
			pathSeparator: "/",
			expected:      "",
		},
		{
			name:          "tilde-expanded path is left unchanged",
			path:          "~/charts",
			volumeName:    "",
			pathSeparator: "/",
			expected:      "~/charts",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result := anchorBareVolumeToRoot(testCase.path, testCase.volumeName, testCase.pathSeparator)
			assert.Equal(t, testCase.expected, result)
		})
	}
}
