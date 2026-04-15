package output

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// ColorsEnabled returns true if terminal colors should be used.
// Respects NO_COLOR environment variable (https://no-color.org/)
func ColorsEnabled() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	if noColor {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // safe fd conversion
}

// ANSI color codes
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

// Symbols for CLI output (ASCII-compatible)
const (
	SymbolSuccess = "+"
	SymbolError   = "x"
	SymbolWarning = "!"
	SymbolInfo    = "*"
	SymbolArrow   = "->"
	SymbolBullet  = "-"
)

// Bold returns text in bold (or plain if colors disabled)
func Bold(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", bold, text, reset)
}

// Dim returns text in dim style (or plain if colors disabled)
func Dim(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", dim, text, reset)
}

// Success returns text styled for success messages
func Success(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", green, text, reset)
}

// Error returns text styled for error messages
func Error(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", red, text, reset)
}

// Warning returns text styled for warning messages
func Warning(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", yellow, text, reset)
}

// Info returns text styled for informational messages
func Info(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s", cyan, text, reset)
}

// Header returns text styled as a section header
func Header(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s%s", bold, white, text, reset)
}

// Secondary returns text in dim cyan for secondary information
func Secondary(text string) string {
	if !ColorsEnabled() {
		return text
	}
	return fmt.Sprintf("%s%s%s%s", dim, cyan, text, reset)
}

// PrintHeader prints a bold section header
func PrintHeader(text string) {
	fmt.Println(Header(text))
}

// PrintSuccess prints a success message with checkmark
func PrintSuccess(message string) {
	fmt.Printf("%s %s\n", Success(SymbolSuccess), Success(message))
}

// PrintError prints an error message with X symbol to stderr
func PrintError(message string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", Error(SymbolError), Error(message))
}

// PrintWarning prints a warning message with ! symbol to stderr
func PrintWarning(message string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", Warning(SymbolWarning), Warning(message))
}

// PrintWarningNewline prints a blank line to stderr for visual spacing in warning/confirmation blocks.
func PrintWarningNewline() {
	fmt.Fprintln(os.Stderr)
}

// PrintWarningDetail prints a detail line to stderr, for supplementary context under a warning.
func PrintWarningDetail(message string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", SymbolBullet, message)
}

// PrintWarningLabel prints a label line to stderr, used as a sub-section heading within warning blocks.
func PrintWarningLabel(label string) {
	fmt.Fprintf(os.Stderr, "  %s\n", label)
}

// PrintWarningBulletField prints an indented bullet item with a right-side value to stderr.
func PrintWarningBulletField(name string, value string) {
	fmt.Fprintf(os.Stderr, "    %s %-36s %s\n", SymbolBullet, name, value)
}

// PrintWarningSecondary prints a secondary/hint line to stderr, for actionable guidance under a warning.
func PrintWarningSecondary(message string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", SymbolArrow, Secondary(message))
}

// PrintInfo prints an info message with * symbol
func PrintInfo(message string) {
	fmt.Printf("%s %s\n", Info(SymbolInfo), Info(message))
}

// PrintStep prints a step being executed with arrow
func PrintStep(message string) {
	fmt.Printf("  %s %s\n", SymbolArrow, message)
}

// PrintSecondary prints secondary/supplementary information
func PrintSecondary(message string) {
	fmt.Printf("  %s %s\n", SymbolArrow, Secondary(message))
}

// PrintNewline prints a blank line to stdout for visual spacing.
func PrintNewline() {
	fmt.Println()
}

// PrintBullet prints a bullet list item to stdout.
func PrintBullet(message string) {
	fmt.Printf("  %s %s\n", SymbolBullet, message)
}

// PrintField prints an indented key-value pair to stdout.
// The label is right-padded to 13 characters for alignment.
func PrintField(label string, value string) {
	fmt.Printf("  %-13s%s\n", label, value)
}

// PrintLabel prints an indented label line to stdout, used as a sub-section heading.
func PrintLabel(label string) {
	fmt.Printf("  %s\n", label)
}

// PrintBulletField prints an indented bullet item with a right-side value.
func PrintBulletField(name string, value string) {
	fmt.Printf("    %s %-36s %s\n", SymbolBullet, name, value)
}

// PrintSubfield prints a deeply-indented key-value pair, used for detail lines under a bullet.
func PrintSubfield(label string, value string) {
	fmt.Printf("      %-7s%s\n", label, value)
}

// Plural returns the singular or plural form based on count
func Plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
