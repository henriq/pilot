package progress

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// terminalCapabilities holds detected terminal features
type terminalCapabilities struct {
	supportsANSI  bool
	terminalWidth int
}

// detectCapabilities returns the terminal capabilities for the current environment.
// It is safe to call multiple times; on Windows, SetConsoleMode is idempotent.
func detectCapabilities() terminalCapabilities {
	width, _, err := term.GetSize(int(os.Stdout.Fd())) //nolint:gosec // safe fd conversion
	if err != nil || width <= 0 {
		width = 80 // fallback default
	}

	supportsANSI := initTerminal()

	return terminalCapabilities{
		supportsANSI:  supportsANSI,
		terminalWidth: width,
	}
}

// clearLine returns the escape sequence to clear the current line
func clearLine(caps terminalCapabilities) string {
	if caps.supportsANSI {
		return "\033[2K\r"
	}
	// Space-padding fallback for terminals without ANSI support
	return "\r" + strings.Repeat(" ", caps.terminalWidth) + "\r"
}

// truncateToWidth truncates a string to fit within the terminal width,
// accounting for ANSI escape sequences (which don't consume visual space).
// Returns the truncated string with ANSI reset appended if truncation occurred.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	visibleLen := 0
	inEscape := false
	truncated := false

	for _, r := range s {
		if r == '\033' {
			inEscape = true
			result.WriteRune(r)
			continue
		}

		if inEscape {
			result.WriteRune(r)
			// ANSI sequences end with a letter (A-Z, a-z)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		// Regular visible character
		if visibleLen >= width {
			truncated = true
			break
		}
		result.WriteRune(r)
		visibleLen++
	}

	// Reset ANSI formatting if we truncated mid-string (and not mid-escape)
	if truncated && !inEscape {
		result.WriteString("\033[0m")
	}

	return result.String()
}
