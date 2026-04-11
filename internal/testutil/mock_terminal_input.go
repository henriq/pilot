package testutil

import (
	"pilot/internal/ports"

	"github.com/stretchr/testify/mock"
)

// Compile-time interface compliance check
var _ ports.TerminalInput = (*MockTerminalInput)(nil)

// MockTerminalInput provides a testify mock for ports.TerminalInput
type MockTerminalInput struct {
	mock.Mock
}

func (m *MockTerminalInput) ReadPassword(prompt string) (string, error) {
	args := m.Called(prompt)
	return args.String(0), args.Error(1)
}

func (m *MockTerminalInput) ReadLine(prompt string) (string, error) {
	args := m.Called(prompt)
	return args.String(0), args.Error(1)
}

func (m *MockTerminalInput) IsTerminal() bool {
	args := m.Called()
	return args.Bool(0)
}
