package scm

import (
	"errors"
	"testing"

	"pilot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitClient_ContainsRepository_True(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(true, nil)

	client := NewGitClient(commandRunner, fileSystem)

	result := client.ContainsRepository("/repo")

	assert.True(t, result)
	fileSystem.AssertExpectations(t)
}

func TestGitClient_ContainsRepository_False(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(false, nil)

	client := NewGitClient(commandRunner, fileSystem)

	result := client.ContainsRepository("/repo")

	assert.False(t, result)
	fileSystem.AssertExpectations(t)
}

func TestGitClient_UpdateOriginUrl_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"remote", "set-url", "origin", "https://github.com/user/repo.git"}).
		Return([]byte(""), nil)

	client := NewGitClient(commandRunner, fileSystem)

	err := client.UpdateOriginUrl("/repo", "https://github.com/user/repo.git")

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestGitClient_UpdateOriginUrl_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"remote", "set-url", "origin", "invalid-url"}).
		Return([]byte("fatal: No such remote 'origin'"), errors.New("exit status 1"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.UpdateOriginUrl("/repo", "invalid-url")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update git remote URL")
	assert.Contains(t, err.Error(), "No such remote")
}

func TestGitClient_FetchRefFromOrigin_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnvInDir", "/repo", sshBatchModeEnv, "git", []string{"-c", "core.autocrlf=false", "fetch", "origin", "-f", "main"}).
		Return([]byte("From https://github.com/user/repo\n * branch main -> FETCH_HEAD"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	err := client.FetchRefFromOrigin("/repo", "main")

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestGitClient_FetchRefFromOrigin_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnvInDir", "/repo", sshBatchModeEnv, "git", []string{"-c", "core.autocrlf=false", "fetch", "origin", "-f", "nonexistent"}).
		Return([]byte("fatal: couldn't find remote ref nonexistent"), errors.New("exit status 1"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.FetchRefFromOrigin("/repo", "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch")
}

func TestGitClient_GetCurrentRef_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}).
		Return([]byte("feature/my-branch\n"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	ref, err := client.GetCurrentRef("/repo")

	require.NoError(t, err)
	assert.Equal(t, "feature/my-branch", ref)
}

func TestGitClient_GetCurrentRef_DetachedHead(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}).
		Return([]byte("HEAD\n"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	ref, err := client.GetCurrentRef("/repo")

	require.NoError(t, err)
	assert.Equal(t, "HEAD", ref)
}

func TestGitClient_GetCurrentRef_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}).
		Return([]byte(""), errors.New("not a git repository"))

	client := NewGitClient(commandRunner, fileSystem)

	_, err := client.GetCurrentRef("/repo")

	require.Error(t, err)
}

func TestGitClient_Checkout_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "checkout", "main"}).
		Return([]byte("Switched to branch 'main'"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	err := client.Checkout("/repo", "main")

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestGitClient_Checkout_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "checkout", "nonexistent"}).
		Return([]byte("error: pathspec 'nonexistent' did not match any file(s) known to git"), errors.New("exit status 1"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.Checkout("/repo", "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to checkout nonexistent")
	assert.Contains(t, err.Error(), "pathspec")
}

func TestGitClient_IsBranch_True(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}).
		Return([]byte("abc123def456"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	result := client.IsBranch("/repo", "main")

	assert.True(t, result)
}

func TestGitClient_IsBranch_False(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/nonexistent"}).
		Return([]byte(""), errors.New("exit status 1"))

	client := NewGitClient(commandRunner, fileSystem)

	result := client.IsBranch("/repo", "nonexistent")

	assert.False(t, result)
}

func TestGitClient_GetRevisionForCommit_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "HEAD"}).
		Return([]byte("abc123def456789\n"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	revision, err := client.GetRevisionForCommit("/repo", "HEAD")

	require.NoError(t, err)
	assert.Equal(t, "abc123def456789\n", revision)
}

func TestGitClient_GetRevisionForCommit_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "invalid-ref"}).
		Return([]byte("fatal: ambiguous argument 'invalid-ref'"), errors.New("exit status 1"))

	client := NewGitClient(commandRunner, fileSystem)

	_, err := client.GetRevisionForCommit("/repo", "invalid-ref")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get origin revision")
}

func TestGitClient_ResetToCommit_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "reset", "--hard", "abc123"}).
		Return([]byte("HEAD is now at abc123 commit message"), nil)

	client := NewGitClient(commandRunner, fileSystem)

	err := client.ResetToCommit("/repo", "abc123")

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestGitClient_ResetToCommit_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "reset", "--hard", "invalid"}).
		Return([]byte("fatal: Could not parse object 'invalid'"), errors.New("exit status 1"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.ResetToCommit("/repo", "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reset to invalid")
}

func TestGitClient_Download_Success(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "https://github.com/user/repo.git", "--branch", "main", "/path/to/dest"}).
		Return([]byte("Cloning into '/path/to/dest'..."), nil)

	client := NewGitClient(commandRunner, fileSystem)

	err := client.Download("/path/to/dest", "main", "https://github.com/user/repo.git")

	require.NoError(t, err)
	commandRunner.AssertExpectations(t)
}

func TestGitClient_Download_Error(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "https://invalid-url", "--branch", "main", "/path/to/dest"}).
		Return([]byte("fatal: repository 'https://invalid-url' not found"), errors.New("exit status 128"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.Download("/path/to/dest", "main", "https://invalid-url")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clone https://invalid-url")
	assert.Contains(t, err.Error(), "repository")
}

// Tests for the Git struct (higher-level orchestrator)

func TestGit_Download_NewRepository_CreatesDirectoryAndClones(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)

	// Repository doesn't exist yet
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(false, nil)

	// Should create the directory
	fileSystem.On("MkdirAll", "/repo", testutil.AnyAccessMode).Return(nil)

	// Should clone the repository
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "https://github.com/user/repo.git", "--branch", "main", "/repo"}).
		Return([]byte("Cloning..."), nil)

	gitClient := NewGitClient(commandRunner, fileSystem)
	git := NewGit(gitClient, fileSystem)

	err := git.Download("https://github.com/user/repo.git", "main", "/repo")

	require.NoError(t, err)
	fileSystem.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
}

func TestGit_Download_ExistingRepository_UpdatesInsteadOfCloning(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)

	// Repository already exists
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(true, nil)

	// Should update origin URL
	commandRunner.On("RunInDir", "/repo", "git", []string{"remote", "set-url", "origin", "https://github.com/user/repo.git"}).
		Return([]byte(""), nil)

	// Should fetch the ref (uses SSH batch mode)
	commandRunner.On("RunWithEnvInDir", "/repo", sshBatchModeEnv, "git", []string{"-c", "core.autocrlf=false", "fetch", "origin", "-f", "main"}).
		Return([]byte(""), nil)

	// Should check current ref (already on main)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}).
		Return([]byte("main\n"), nil)

	// Should check if it's a branch
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}).
		Return([]byte("abc123"), nil)

	// Should get local and origin revisions to compare
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "main"}).
		Return([]byte("abc123\n"), nil)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "origin/main"}).
		Return([]byte("abc123\n"), nil)

	gitClient := NewGitClient(commandRunner, fileSystem)
	git := NewGit(gitClient, fileSystem)

	err := git.Download("https://github.com/user/repo.git", "main", "/repo")

	require.NoError(t, err)
	fileSystem.AssertExpectations(t)
	commandRunner.AssertExpectations(t)
}

func TestGit_Download_Deduplication_SameRepoRefNotClonedTwice(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)

	// Repository doesn't exist
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(false, nil).Once()

	// Should create directory and clone only once
	fileSystem.On("MkdirAll", "/repo", testutil.AnyAccessMode).Return(nil).Once()
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "https://github.com/user/repo.git", "--branch", "main", "/repo"}).
		Return([]byte("Cloning..."), nil).Once()

	gitClient := NewGitClient(commandRunner, fileSystem)
	git := NewGit(gitClient, fileSystem)

	// First download
	err := git.Download("https://github.com/user/repo.git", "main", "/repo")
	require.NoError(t, err)

	// Second download with same repo+ref should not clone again
	err = git.Download("https://github.com/user/repo.git", "main", "/repo")
	require.NoError(t, err)

	// Verify clone was only called once
	commandRunner.AssertNumberOfCalls(t, "RunWithEnv", 1)
}

func TestGit_Download_DifferentRefs_BothDownloaded(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)

	// First download: repo doesn't exist
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(false, nil).Once()
	fileSystem.On("MkdirAll", "/repo", testutil.AnyAccessMode).Return(nil).Once()
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "https://github.com/user/repo.git", "--branch", "main", "/repo"}).
		Return([]byte("Cloning..."), nil).Once()

	// Second download with different ref: repo now exists
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(true, nil).Once()
	commandRunner.On("RunInDir", "/repo", "git", []string{"remote", "set-url", "origin", "https://github.com/user/repo.git"}).
		Return([]byte(""), nil)
	commandRunner.On("RunWithEnvInDir", "/repo", sshBatchModeEnv, "git", []string{"-c", "core.autocrlf=false", "fetch", "origin", "-f", "feature"}).
		Return([]byte(""), nil)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}).
		Return([]byte("main\n"), nil)
	commandRunner.On("RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "checkout", "feature"}).
		Return([]byte(""), nil)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/feature"}).
		Return([]byte(""), errors.New("not a branch")) // Tag, not branch

	gitClient := NewGitClient(commandRunner, fileSystem)
	git := NewGit(gitClient, fileSystem)

	// First download with main
	err := git.Download("https://github.com/user/repo.git", "main", "/repo")
	require.NoError(t, err)

	// Second download with feature (different ref)
	err = git.Download("https://github.com/user/repo.git", "feature", "/repo")
	require.NoError(t, err)

	// Both should have been processed
	fileSystem.AssertNumberOfCalls(t, "FileExists", 2)
}

func TestGit_Download_ExistingRepo_ResetsToBranch_WhenBehindOrigin(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)

	// Repository exists
	fileSystem.On("FileExists", "/repo/.git/HEAD").Return(true, nil)

	// Update origin
	commandRunner.On("RunInDir", "/repo", "git", []string{"remote", "set-url", "origin", "https://github.com/user/repo.git"}).
		Return([]byte(""), nil)

	// Fetch (uses SSH batch mode)
	commandRunner.On("RunWithEnvInDir", "/repo", sshBatchModeEnv, "git", []string{"-c", "core.autocrlf=false", "fetch", "origin", "-f", "main"}).
		Return([]byte(""), nil)

	// Already on main
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}).
		Return([]byte("main\n"), nil)

	// It's a branch
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}).
		Return([]byte("abc123"), nil)

	// Local is behind origin (different revisions)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "main"}).
		Return([]byte("old123\n"), nil)
	commandRunner.On("RunInDir", "/repo", "git", []string{"rev-parse", "origin/main"}).
		Return([]byte("new456\n"), nil)

	// Should reset to origin/main
	commandRunner.On("RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "reset", "--hard", "origin/main"}).
		Return([]byte("HEAD is now at new456"), nil)

	gitClient := NewGitClient(commandRunner, fileSystem)
	git := NewGit(gitClient, fileSystem)

	err := git.Download("https://github.com/user/repo.git", "main", "/repo")

	require.NoError(t, err)
	commandRunner.AssertCalled(t, "RunInDir", "/repo", "git", []string{"-c", "core.autocrlf=false", "reset", "--hard", "origin/main"})
}

// SSH Authentication Error Tests

func TestGitClient_Download_SSHAuthError_ProvidesHelpfulMessage(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "git@github.com:user/repo.git", "--branch", "main", "/path/to/dest"}).
		Return([]byte("Permission denied (publickey)."), errors.New("exit status 128"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.Download("/path/to/dest", "main", "git@github.com:user/repo.git")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH authentication failed")
	assert.Contains(t, err.Error(), "ssh-agent")
	assert.Contains(t, err.Error(), "ssh-add")
}

func TestGitClient_Download_SSHHostKeyError_ProvidesHelpfulMessage(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnv", "git", sshBatchModeEnv, []string{"clone", "-c", "core.autocrlf=false", "git@github.com:user/repo.git", "--branch", "main", "/path/to/dest"}).
		Return([]byte("Host key verification failed."), errors.New("exit status 128"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.Download("/path/to/dest", "main", "git@github.com:user/repo.git")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH host key verification failed")
	assert.Contains(t, err.Error(), "known_hosts")
	assert.Contains(t, err.Error(), "ssh -T git@github.com")
	assert.Contains(t, err.Error(), "ssh -T git@gitlab.com")
}

func TestGitClient_FetchRefFromOrigin_SSHAuthError_ProvidesHelpfulMessage(t *testing.T) {
	commandRunner := new(testutil.MockCommandRunner)
	fileSystem := new(testutil.MockFileSystem)
	commandRunner.On("RunWithEnvInDir", "/repo", sshBatchModeEnv, "git", []string{"-c", "core.autocrlf=false", "fetch", "origin", "-f", "main"}).
		Return([]byte("Permission denied (publickey)."), errors.New("exit status 128"))

	client := NewGitClient(commandRunner, fileSystem)

	err := client.FetchRefFromOrigin("/repo", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH authentication failed")
	assert.Contains(t, err.Error(), "ssh-agent")
}

func TestIsSSHAuthError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{"Permission denied", "Permission denied (publickey).", true},
		{"Connection closed", "Connection closed by remote host", true},
		{"Connection reset", "Connection reset by peer", true},
		{"Host key verification - not auth error", "Host key verification failed.", false},
		{"Regular error", "fatal: repository not found", false},
		{"Empty output", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSSHAuthError(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHostKeyError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{"Host key verification", "Host key verification failed.", true},
		{"Permission denied - not host key", "Permission denied (publickey).", false},
		{"Regular error", "fatal: repository not found", false},
		{"Empty output", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHostKeyError(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}
