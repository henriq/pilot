package main

import (
	"os"
	"testing"
)

func TestAppStarts(t *testing.T) {
	// Save original args
	oldArgs := os.Args

	// Restore original args when test completes
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"pilot", "help"}
	// Execute the main function
	main()
	// Check if the command executed successfully
	if err := recover(); err != nil {
		t.Errorf("App failed to start: %v", err)
	}
}
