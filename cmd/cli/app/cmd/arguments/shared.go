// Package arguments parses and completes pilot's CLI flags and positional args.
//
// Override flags pair a name with a value using ":" rather than "=", because
// Cobra's zsh and fish completion scripts run a `-.*=` regex that mangles
// values containing a dash.
package arguments

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pilot/internal/core/domain"

	"github.com/spf13/cobra"
)

func expandLeadingTilde(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	expanded := filepath.Join(home, path[2:])
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, `\`) {
		expanded += string(filepath.Separator)
	}
	return expanded, nil
}

// On Windows a bare "C:" means "current directory on drive C:" rather than the
// drive root; appending the separator forces filepath.Split to treat it as the root.
func anchorBareVolumeToRoot(path, volumeName, pathSeparator string) string {
	if volumeName != "" && path == volumeName {
		return path + pathSeparator
	}
	return path
}

func prefixedDirectoryCompletions(entryPrefix, pathToComplete string) []cobra.Completion {
	if pathToComplete == "~" {
		pathToComplete = "~" + string(filepath.Separator)
	}
	pathToComplete = anchorBareVolumeToRoot(
		pathToComplete,
		filepath.VolumeName(pathToComplete),
		string(filepath.Separator),
	)
	directoryToList, fileNamePrefix := filepath.Split(pathToComplete)
	listTarget, err := expandLeadingTilde(directoryToList)
	if err != nil {
		return nil
	}
	if listTarget == "" {
		listTarget = "."
	}
	entries, err := os.ReadDir(listTarget)
	if err != nil {
		return nil
	}
	completions := make([]cobra.Completion, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), fileNamePrefix) {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(fileNamePrefix, ".") {
			continue
		}
		// Cobra's completion protocol is newline-delimited, so a name carrying a
		// newline would split into two bogus candidates.
		if strings.IndexFunc(entry.Name(), isControlCharacter) >= 0 {
			continue
		}
		entryInfo, err := os.Stat(filepath.Join(listTarget, entry.Name()))
		if err != nil || !entryInfo.IsDir() {
			continue
		}
		completions = append(completions, entryPrefix+directoryToList+entry.Name()+string(filepath.Separator))
	}
	return completions
}

func serviceNameColonCompletions(services []domain.Service) []cobra.Completion {
	completions := make([]cobra.Completion, 0, len(services))
	for _, service := range services {
		completions = append(completions, service.Name+":")
	}
	return completions
}

type nameValueFlag struct {
	flagName       string
	formatHint     string
	nameNoun       string
	valueNoun      string
	normalizeValue func(string) (string, error)
}

// parse splits repeated "name:value" entries into a map. Duplicate names take
// the last value.
func (f nameValueFlag) parse(entries []string) (map[string]string, error) {
	valueByName := map[string]string{}
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, ":")
		if !found {
			return nil, fmt.Errorf(
				"invalid --%s value %q; expected format: %s",
				f.flagName, entry, f.formatHint,
			)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			return nil, fmt.Errorf("invalid --%s value %q; %s name cannot be empty", f.flagName, entry, f.nameNoun)
		}
		if value == "" {
			return nil, fmt.Errorf("invalid --%s value %q; %s cannot be empty", f.flagName, entry, f.valueNoun)
		}
		normalized, err := f.normalizeValue(value)
		if err != nil {
			return nil, fmt.Errorf("invalid --%s value %q; %w", f.flagName, entry, err)
		}
		valueByName[name] = normalized
	}
	return valueByName, nil
}

func isControlCharacter(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// The control-character scan runs on the raw pre-expansion path so a hostile
// value is rejected before any tilde expansion or stat.
func resolveExistingDirectory(rawPath string) (string, error) {
	if index := strings.IndexFunc(rawPath, isControlCharacter); index >= 0 {
		return "", fmt.Errorf("path contains a control character at byte %d", index)
	}
	expandedPath, err := expandLeadingTilde(rawPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", rawPath, err)
	}
	absolutePath, err := filepath.Abs(expandedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", rawPath, err)
	}
	info, err := os.Stat(absolutePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("path %q does not exist", absolutePath)
	}
	if err != nil {
		return "", fmt.Errorf("path %q: %w", absolutePath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", absolutePath)
	}
	// Resolve symlinks so the path handed to docker build or helm cannot be
	// redirected by swapping a link between this check and the build itself.
	realPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return "", fmt.Errorf("path %q: %w", absolutePath, err)
	}
	return realPath, nil
}
