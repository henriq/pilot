package scm

import (
	"fmt"
	"path/filepath"
	"strings"

	"pilot/internal/ports"
)

// sshBatchModeEnv configures SSH to fail immediately instead of hanging
// when authentication requires user input (e.g., password-protected keys).
var sshBatchModeEnv = []string{"GIT_SSH_COMMAND=ssh -o BatchMode=yes"}

// isSSHAuthError checks if the error output indicates an SSH authentication failure
// (permission denied or connection issues, but NOT host key verification).
func isSSHAuthError(output string) bool {
	return strings.Contains(output, "Permission denied") ||
		strings.Contains(output, "Connection closed by remote host") ||
		strings.Contains(output, "Connection reset by peer")
}

// isHostKeyError checks if the error output indicates a host key verification failure.
func isHostKeyError(output string) bool {
	return strings.Contains(output, "Host key verification failed")
}

// wrapSSHAuthError wraps an error with helpful SSH troubleshooting information.
func wrapSSHAuthError(operation, url string, output []byte, err error) error {
	outputStr := string(output)
	if isSSHAuthError(outputStr) {
		return fmt.Errorf("SSH authentication failed during %s for %s.\n\n"+
			"If your SSH key has a passphrase, add it to ssh-agent first:\n"+
			"  eval \"$(ssh-agent -s)\"\n"+
			"  ssh-add ~/.ssh/id_rsa\n\n"+
			"Original error: %s", operation, url, outputStr)
	}
	if isHostKeyError(outputStr) {
		return fmt.Errorf("SSH host key verification failed during %s for %s.\n\n"+
			"The remote host is not in your known_hosts file. Connect once manually to add it:\n"+
			"  ssh -T git@github.com    # For GitHub\n"+
			"  ssh -T git@gitlab.com    # For GitLab\n\n"+
			"Accept the host key when prompted, then retry your pilot command.\n\n"+
			"Original error: %s", operation, url, outputStr)
	}
	return fmt.Errorf("failed to %s %s: %v\n%s", operation, url, err, outputStr)
}

type GitClient struct {
	commandRunner ports.CommandRunner
	fileSystem    ports.FileSystem
}

func NewGitClient(commandRunner ports.CommandRunner, fileSystem ports.FileSystem) *GitClient {
	return &GitClient{
		commandRunner: commandRunner,
		fileSystem:    fileSystem,
	}
}

func (g *GitClient) ContainsRepository(repositoryPath string) bool {
	exists, err := g.fileSystem.FileExists(filepath.Join(repositoryPath, ".git", "HEAD"))
	return err == nil && exists
}

func (g *GitClient) UpdateOriginUrl(repositoryPath string, originUrl string) error {
	output, err := g.commandRunner.RunInDir(repositoryPath, "git", "remote", "set-url", "origin", originUrl)
	if err != nil {
		return fmt.Errorf("failed to update git remote URL: %v\n%s", err, string(output))
	}

	return nil
}

func (g *GitClient) FetchRefFromOrigin(repositoryPath string, branch string) error {
	output, err := g.commandRunner.RunWithEnvInDir(repositoryPath, sshBatchModeEnv, "git", "-c", "core.autocrlf=false", "fetch", "origin", "-f", branch)
	if err != nil {
		return wrapSSHAuthError("fetch", "origin/"+branch, output, err)
	}

	return nil
}

func (g *GitClient) GetCurrentRef(repositoryPath string) (string, error) {
	output, err := g.commandRunner.RunInDir(repositoryPath, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (g *GitClient) Checkout(repositoryPath string, commit string) error {
	output, err := g.commandRunner.RunInDir(repositoryPath, "git", "-c", "core.autocrlf=false", "checkout", commit)
	if err != nil {
		return fmt.Errorf("failed to checkout %s: %v\n%s", commit, err, string(output))
	}

	return nil
}

func (g *GitClient) IsBranch(repositoryPath string, branch string) bool {
	_, err := g.commandRunner.RunInDir(repositoryPath, "git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return err == nil
}

func (g *GitClient) GetRevisionForCommit(repositoryPath string, commit string) (string, error) {
	output, err := g.commandRunner.RunInDir(repositoryPath, "git", "rev-parse", commit)
	if err != nil {
		return "", fmt.Errorf("failed to get origin revision: %v\n%s", err, string(output))
	}

	return string(output), nil
}

func (g *GitClient) ResetToCommit(repositoryPath string, commit string) error {
	output, err := g.commandRunner.RunInDir(repositoryPath, "git", "-c", "core.autocrlf=false", "reset", "--hard", commit)
	if err != nil {
		return fmt.Errorf("failed to reset to %s: %v\n%s", commit, err, string(output))
	}

	return nil
}

func (g *GitClient) Download(repositoryPath string, branch string, repositoryUrl string) error {
	output, err := g.commandRunner.RunWithEnv("git", sshBatchModeEnv, "clone", "-c", "core.autocrlf=false", repositoryUrl, "--branch", branch, repositoryPath)
	if err != nil {
		return wrapSSHAuthError("clone", repositoryUrl, output, err)
	}

	return nil
}
