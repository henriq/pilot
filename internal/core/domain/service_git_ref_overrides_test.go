package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustServiceGitRefOverrides(t *testing.T, gitRefByService map[string]string) ServiceGitRefOverrides {
	t.Helper()
	overrides, err := NewServiceGitRefOverrides(gitRefByService)
	require.NoError(t, err)
	return overrides
}

func TestServiceGitRefOverrides_LookupGitRef_ReturnsConfiguredRef(t *testing.T) {
	overrides := mustServiceGitRefOverrides(t, map[string]string{"api": "feature/x"})

	gitRef, ok := overrides.LookupGitRef("api")

	require.True(t, ok)
	assert.Equal(t, "feature/x", gitRef)
}

func TestServiceGitRefOverrides_LookupGitRef_AbsentForUnconfiguredService(t *testing.T) {
	overrides := mustServiceGitRefOverrides(t, map[string]string{"api": "feature/x"})

	_, ok := overrides.LookupGitRef("worker")

	assert.False(t, ok)
}

func TestServiceGitRefOverrides_ValidateAgainstServices_AllowsKnownService(t *testing.T) {
	services := []Service{{Name: "api"}, {Name: "worker"}}
	overrides := mustServiceGitRefOverrides(t, map[string]string{"api": "feature/x"})

	require.NoError(t, overrides.ValidateAgainstServices(services))
}

func TestServiceGitRefOverrides_ValidateAgainstServices_RejectsUnknownService(t *testing.T) {
	services := []Service{{Name: "api"}, {Name: "worker"}}
	overrides := mustServiceGitRefOverrides(t, map[string]string{"frontend": "feature/x"})

	err := overrides.ValidateAgainstServices(services)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontend")
	assert.Contains(t, err.Error(), "api")
	assert.Contains(t, err.Error(), "worker")
	assert.NotContains(t, err.Error(), "--git-ref", "domain error must not leak the CLI flag name")
}

func TestServiceGitRefOverrides_ValidateAgainstServices_ListsAvailableServicesAsSortedBullets(t *testing.T) {
	services := []Service{{Name: "worker"}, {Name: "api"}, {Name: "frontend"}}
	overrides := mustServiceGitRefOverrides(t, map[string]string{"missing": "feature/x"})

	err := overrides.ValidateAgainstServices(services)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "available services:\n  - api\n  - frontend\n  - worker")
}

func TestServiceGitRefOverrides_ValidateAgainstServices_EmptyIsNoop(t *testing.T) {
	services := []Service{{Name: "api"}}
	overrides := mustServiceGitRefOverrides(t, nil)

	require.NoError(t, overrides.ValidateAgainstServices(services))
}

func TestServiceGitRefOverrides_FindUnusedOverrides_ReturnsNamesNotInScope(t *testing.T) {
	servicesInScope := []Service{{Name: "api"}}
	overrides := mustServiceGitRefOverrides(t, map[string]string{
		"api":      "main",
		"worker":   "develop",
		"frontend": "feature/x",
	})

	unused := overrides.FindUnusedOverrides(servicesInScope)

	assert.Equal(t, []string{"frontend", "worker"}, unused)
}

func TestServiceGitRefOverrides_FindUnusedOverrides_EmptyWhenAllInScope(t *testing.T) {
	servicesInScope := []Service{{Name: "api"}, {Name: "worker"}}
	overrides := mustServiceGitRefOverrides(t, map[string]string{"api": "main", "worker": "develop"})

	assert.Empty(t, overrides.FindUnusedOverrides(servicesInScope))
}

func TestServiceGitRefOverrides_FindUnusedOverrides_NilWhenEmpty(t *testing.T) {
	overrides := mustServiceGitRefOverrides(t, nil)
	assert.Nil(t, overrides.FindUnusedOverrides([]Service{{Name: "api"}}))
}

func TestServiceGitRefOverrides_NewServiceGitRefOverrides_IsolatesFromInputMutation(t *testing.T) {
	input := map[string]string{"api": "main"}
	overrides, err := NewServiceGitRefOverrides(input)
	require.NoError(t, err)

	input["api"] = "mutated"

	gitRef, ok := overrides.LookupGitRef("api")
	require.True(t, ok)
	assert.Equal(t, "main", gitRef)
}

func TestServiceGitRefOverrides_NewServiceGitRefOverrides_RejectsInvalidRefShape(t *testing.T) {
	_, err := NewServiceGitRefOverrides(map[string]string{"api": "--upload-pack=/tmp/x.sh"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"api"`)
	assert.Contains(t, err.Error(), "cannot start with '-'")
}

func TestValidateGitRefShape(t *testing.T) {
	cases := map[string]struct {
		gitRef    string
		expectErr string
	}{
		"plain branch":   {"main", ""},
		"slash in ref":   {"release/1.2.3", ""},
		"version tag":    {"v1.2.3", ""},
		"commit sha":     {"a1b2c3d4e5f6", ""},
		"empty":          {"", "cannot be empty"},
		"leading dash":   {"--upload-pack=x", "cannot start with '-'"},
		"leading slash":  {"/main", "cannot start or end with '/'"},
		"trailing slash": {"main/", "cannot start or end with '/'"},
		"lock suffix":    {"main.lock", "cannot end with '.lock'"},
		"double dot":     {"feat..main", "cannot contain '..'"},
		"at brace":       {"main@{0}", "cannot contain '@{'"},
		"space":          {"feature branch", "disallowed character"},
		"tab":            {"feature\tbranch", "disallowed character"},
		"tilde":          {"main~1", "disallowed character"},
		"caret":          {"main^", "disallowed character"},
		"colon":          {"evil:refs/heads/main", "disallowed character"},
		"question mark":  {"main?", "disallowed character"},
		"asterisk":       {"feat*", "disallowed character"},
		"open bracket":   {"main[0]", "disallowed character"},
		"backslash":      {"feat\\main", "disallowed character"},
		"control char":   {"main\x1b", "control character"},
		"del char":       {"main\x7f", "control character"},
	}
	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			err := ValidateGitRefShape(testCase.gitRef)
			if testCase.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.expectErr)
		})
	}
}

func TestServiceGitRefOverrides_IsEmpty(t *testing.T) {
	cases := map[string]struct {
		gitRefByService map[string]string
		expectEmpty     bool
	}{
		"nil map":       {nil, true},
		"empty map":     {map[string]string{}, true},
		"non-empty map": {map[string]string{"api": "main"}, false},
	}
	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			assert.Equal(t, testCase.expectEmpty, mustServiceGitRefOverrides(t, testCase.gitRefByService).IsEmpty())
		})
	}
}
