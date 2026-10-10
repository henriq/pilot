package arguments

import (
	"os"
	"path/filepath"
	"testing"

	"pilot/internal/core/domain"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageSourceOverrides_ResolvesAbsolutePath(t *testing.T) {
	sourceDirectory := t.TempDir()

	overrides, err := ParseImageSourceOverrides([]string{"api:" + sourceDirectory})
	require.NoError(t, err)

	path, ok := overrides.LookupSourcePath("api")
	require.True(t, ok)
	assert.Equal(t, sourceDirectory, path)
}

func TestParseImageSourceOverrides_EmptyInputIsEmpty(t *testing.T) {
	overrides, err := ParseImageSourceOverrides(nil)
	require.NoError(t, err)
	assert.True(t, overrides.IsEmpty())
}

func TestParseImageSourceOverrides_RejectsMissingSeparator(t *testing.T) {
	_, err := ParseImageSourceOverrides([]string{"api"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image:path")
	assert.Contains(t, err.Error(), `"api"`)
}

func TestParseImageSourceOverrides_RejectsEmptyImage(t *testing.T) {
	sourceDirectory := t.TempDir()

	_, err := ParseImageSourceOverrides([]string{":" + sourceDirectory})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image name cannot be empty")
}

func TestParseImageSourceOverrides_RejectsEmptyPath(t *testing.T) {
	_, err := ParseImageSourceOverrides([]string{"api:"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source path cannot be empty")
}

func TestParseImageSourceOverrides_RejectsMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := ParseImageSourceOverrides([]string{"api:" + missing})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestParseImageSourceOverrides_RejectsControlCharacterInPath(t *testing.T) {
	for _, value := range []string{"api:\x1b[31m/src", "api:\x7f/src"} {
		_, err := ParseImageSourceOverrides([]string{value})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "control character")
	}
}

func TestParseImageSourceOverrides_RejectsFileInsteadOfDirectory(t *testing.T) {
	tempDirectory := t.TempDir()
	filePath := filepath.Join(tempDirectory, "Dockerfile")
	require.NoError(t, os.WriteFile(filePath, []byte("FROM scratch"), 0o600))

	_, err := ParseImageSourceOverrides([]string{"api:" + filePath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestParseImageSourceOverrides_LastWinsOnDuplicateImage(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()

	overrides, err := ParseImageSourceOverrides([]string{"api:" + first, "api:" + second})
	require.NoError(t, err)

	path, ok := overrides.LookupSourcePath("api")
	require.True(t, ok)
	assert.Equal(t, second, path)
}

func TestImageSourceOverrideImageCompletions_OffersImagesWithColonSuffix(t *testing.T) {
	services := []domain.Service{
		{Name: "svc-a", DockerImages: []domain.DockerImage{{Name: "image-a"}, {Name: "image-b"}}},
		{Name: "svc-b", DockerImages: []domain.DockerImage{{Name: "image-a"}, {Name: "image-c"}}},
	}

	completions := imageSourceOverrideImageCompletions(services)

	assert.ElementsMatch(t, []cobra.Completion{"image-a:", "image-b:", "image-c:"}, completions)
}
