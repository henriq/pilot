package arguments

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitRefOverrideCompletion_YieldsNoCompletionsAfterSeparator(t *testing.T) {
	completions, directive := GitRefOverrideCompletion(nil, nil, "api:feat")

	assert.Nil(t, completions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

func TestParseServiceGitRefOverrides_ReturnsRefForKnownService(t *testing.T) {
	overrides, err := ParseServiceGitRefOverrides([]string{"api:feature/my-change"})
	require.NoError(t, err)

	gitRef, ok := overrides.LookupGitRef("api")
	require.True(t, ok)
	assert.Equal(t, "feature/my-change", gitRef)
}

func TestParseServiceGitRefOverrides_EmptyInputIsEmpty(t *testing.T) {
	overrides, err := ParseServiceGitRefOverrides(nil)
	require.NoError(t, err)
	assert.True(t, overrides.IsEmpty())
}

func TestParseServiceGitRefOverrides_RejectsMissingSeparator(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service:ref")
	assert.Contains(t, err.Error(), `"api"`)
}

func TestParseServiceGitRefOverrides_RejectsEmptyService(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{":main"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service name cannot be empty")
}

func TestParseServiceGitRefOverrides_RejectsEmptyRef(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git ref cannot be empty")
}

func TestParseServiceGitRefOverrides_RejectsLeadingDash(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:--upload-pack=/tmp/x.sh"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start with '-'")
}

func TestParseServiceGitRefOverrides_RejectsLeadingSlash(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:/main"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start or end with '/'")
}

func TestParseServiceGitRefOverrides_RejectsTrailingSlash(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:main/"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start or end with '/'")
}

func TestParseServiceGitRefOverrides_RejectsTrailingDotLock(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:main.lock"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot end with '.lock'")
}

func TestParseServiceGitRefOverrides_RejectsDoubleDot(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:feat..main"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot contain '..'")
}

func TestParseServiceGitRefOverrides_RejectsAtBrace(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:main@{0}"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot contain '@{'")
}

func TestParseServiceGitRefOverrides_RejectsDisallowedCharacters(t *testing.T) {
	cases := []struct {
		name string
		ref  string
	}{
		{"space", "feature branch"},
		{"tab", "feature\tbranch"},
		{"tilde", "main~1"},
		{"caret", "main^"},
		{"colon", "main:foo"},
		{"question", "main?"},
		{"asterisk", "main*"},
		{"open bracket", "main["},
		{"backslash", `main\foo`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := ParseServiceGitRefOverrides([]string{"api:" + testCase.ref})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "disallowed character")
		})
	}
}

func TestParseServiceGitRefOverrides_RejectsControlCharacter(t *testing.T) {
	_, err := ParseServiceGitRefOverrides([]string{"api:main\x1b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control character")
}

func TestParseServiceGitRefOverrides_LastWinsOnDuplicateService(t *testing.T) {
	overrides, err := ParseServiceGitRefOverrides([]string{"api:main", "api:feature/x"})
	require.NoError(t, err)

	gitRef, ok := overrides.LookupGitRef("api")
	require.True(t, ok)
	assert.Equal(t, "feature/x", gitRef)
}

func TestParseServiceGitRefOverrides_TrimsWhitespace(t *testing.T) {
	overrides, err := ParseServiceGitRefOverrides([]string{"  api  :  feature/x  "})
	require.NoError(t, err)

	gitRef, ok := overrides.LookupGitRef("api")
	require.True(t, ok)
	assert.Equal(t, "feature/x", gitRef)
}

func TestParseServiceGitRefOverrides_AcceptsSlashInRef(t *testing.T) {
	overrides, err := ParseServiceGitRefOverrides([]string{"api:release/1.2.3"})
	require.NoError(t, err)

	gitRef, ok := overrides.LookupGitRef("api")
	require.True(t, ok)
	assert.Equal(t, "release/1.2.3", gitRef)
}
