package arguments

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHelmChartOverrides_ResolvesAbsolutePath(t *testing.T) {
	chartDirectory := t.TempDir()

	overrides, err := ParseHelmChartOverrides([]string{"api:" + chartDirectory})
	require.NoError(t, err)

	directory, ok := overrides.LookupChartDirectory("api")
	require.True(t, ok)
	assert.Equal(t, chartDirectory, directory)
}

func TestParseHelmChartOverrides_EmptyInputIsEmpty(t *testing.T) {
	overrides, err := ParseHelmChartOverrides(nil)
	require.NoError(t, err)
	assert.True(t, overrides.IsEmpty())
}

func TestParseHelmChartOverrides_RejectsMissingSeparator(t *testing.T) {
	_, err := ParseHelmChartOverrides([]string{"api"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service:path")
	assert.Contains(t, err.Error(), `"api"`)
}

func TestParseHelmChartOverrides_RejectsEmptyService(t *testing.T) {
	chartDirectory := t.TempDir()

	_, err := ParseHelmChartOverrides([]string{":" + chartDirectory})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service name cannot be empty")
}

func TestParseHelmChartOverrides_RejectsEmptyPath(t *testing.T) {
	_, err := ParseHelmChartOverrides([]string{"api:"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart path cannot be empty")
}

func TestParseHelmChartOverrides_RejectsMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := ParseHelmChartOverrides([]string{"api:" + missing})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestParseHelmChartOverrides_RejectsControlCharacterInPath(t *testing.T) {
	for _, value := range []string{"api:\x1b[31m/charts", "api:\x7f/charts"} {
		_, err := ParseHelmChartOverrides([]string{value})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "control character")
	}
}

func TestParseHelmChartOverrides_RejectsFileInsteadOfDirectory(t *testing.T) {
	tempDirectory := t.TempDir()
	filePath := filepath.Join(tempDirectory, "Chart.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("name: x"), 0o600))

	_, err := ParseHelmChartOverrides([]string{"api:" + filePath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestParseHelmChartOverrides_LastWinsOnDuplicateService(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()

	overrides, err := ParseHelmChartOverrides([]string{"api:" + first, "api:" + second})
	require.NoError(t, err)

	directory, ok := overrides.LookupChartDirectory("api")
	require.True(t, ok)
	assert.Equal(t, second, directory)
}

func TestParseHelmChartOverrides_ExpandsLeadingTilde(t *testing.T) {
	homeDirectory := t.TempDir()
	chartDirectoryName := "charts"
	require.NoError(t, os.Mkdir(filepath.Join(homeDirectory, chartDirectoryName), 0o750))
	t.Setenv("HOME", homeDirectory)
	t.Setenv("USERPROFILE", homeDirectory)

	overrides, err := ParseHelmChartOverrides([]string{"api:~/" + chartDirectoryName})
	require.NoError(t, err)

	directory, ok := overrides.LookupChartDirectory("api")
	require.True(t, ok)
	assert.Equal(t, filepath.Join(homeDirectory, chartDirectoryName), directory)
}

func TestHelmChartOverrideCompletion_ReturnsDirectoryCompletionsAfterSeparator(t *testing.T) {
	rootDirectory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDirectory, "charts"), 0o750))

	completions, directive := HelmChartOverrideCompletion(nil, nil, "api:"+rootDirectory+string(filepath.Separator))

	assert.Equal(t, []cobra.Completion{
		"api:" + rootDirectory + string(filepath.Separator) + "charts" + string(filepath.Separator),
	}, completions)
	assert.Equal(t, cobra.ShellCompDirectiveNoSpace, directive)
}
