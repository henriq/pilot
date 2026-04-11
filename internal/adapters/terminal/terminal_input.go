package terminal

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"pilot/internal/ports"

	"golang.org/x/term"
)

// Compile-time interface compliance check
var _ ports.TerminalInput = (*TerminalInput)(nil)

// TerminalInput provides terminal input operations using golang.org/x/term.
type TerminalInput struct{}

// NewTerminalInput creates a new TerminalInput adapter.
func NewTerminalInput() *TerminalInput {
	return &TerminalInput{}
}

// ReadPassword prompts for a password and returns the input without echoing to the terminal.
func (t *TerminalInput) ReadPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // safe fd conversion
	fmt.Println()                                          // Print newline after password input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(password), nil
}

// ReadLine prompts the user and returns the input line.
func (t *TerminalInput) ReadLine(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// IsTerminal returns true if stdin is connected to a terminal.
func (t *TerminalInput) IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) //nolint:gosec // safe fd conversion
}
