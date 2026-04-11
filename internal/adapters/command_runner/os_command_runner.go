package command_runner

import (
	"io"
	"os"
	"os/exec"

	"pilot/internal/ports"
)

// OsCommandRunner executes shell commands using os/exec.
type OsCommandRunner struct{}

func NewOsCommandRunner() *OsCommandRunner {
	return &OsCommandRunner{}
}

func (r *OsCommandRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...) //nolint:gosec // CommandRunner adapter — executing variable commands is its purpose
	return cmd.CombinedOutput()
}

func (r *OsCommandRunner) RunWithEnv(name string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)     //nolint:gosec // CommandRunner adapter — executing variable commands is its purpose
	cmd.Env = append(os.Environ(), env...) // Extend environment instead of replacing
	return cmd.CombinedOutput()
}

func (r *OsCommandRunner) RunInDir(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...) //nolint:gosec // CommandRunner adapter — executing variable commands is its purpose
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func (r *OsCommandRunner) RunWithEnvInDir(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...) //nolint:gosec // CommandRunner adapter — executing variable commands is its purpose
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

func (r *OsCommandRunner) RunWithStdin(stdin io.Reader, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...) //nolint:gosec // CommandRunner adapter — executing variable commands is its purpose
	cmd.Stdin = stdin
	return cmd.CombinedOutput()
}

func (r *OsCommandRunner) RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // CommandRunner adapter — executing variable commands is its purpose
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var _ ports.CommandRunner = (*OsCommandRunner)(nil)
