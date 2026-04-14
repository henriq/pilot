package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestSkipMigration(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *cobra.Command
		expected bool
	}{
		{
			name: "ReturnsTrueForCompletionCommand",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "pilot"}
				completion := &cobra.Command{Use: "completion"}
				root.AddCommand(completion)
				return completion
			},
			expected: true,
		},
		{
			name: "ReturnsTrueForCompletionSubcommand",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "pilot"}
				completion := &cobra.Command{Use: "completion"}
				bash := &cobra.Command{Use: "bash"}
				root.AddCommand(completion)
				completion.AddCommand(bash)
				return bash
			},
			expected: true,
		},
		{
			name: "ReturnsFalseForRegularCommand",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "pilot"}
				install := &cobra.Command{Use: "install"}
				root.AddCommand(install)
				return install
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setup()
			assert.Equal(t, tt.expected, skipMigration(cmd))
		})
	}
}
