package handler

import (
	"fmt"
	"path/filepath"

	"pilot/internal/cli/output"
	"pilot/internal/ports"
)

func isStateEntry(name string) bool {
	switch name {
	case "ca", "secrets", "env-key":
		return true
	default:
		return false
	}
}

type CacheCommandHandler struct {
	configRepository ports.ConfigRepository
	fileSystem       ports.FileSystem
	terminalInput    ports.TerminalInput
}

func NewCacheCommandHandler(
	configRepository ports.ConfigRepository,
	fileSystem ports.FileSystem,
	terminalInput ports.TerminalInput,
) CacheCommandHandler {
	return CacheCommandHandler{
		configRepository: configRepository,
		fileSystem:       fileSystem,
		terminalInput:    terminalInput,
	}
}

type cacheEntry struct {
	name string
	path string
	size int64
}

type contextStatus struct {
	name    string
	path    string
	entries []cacheEntry
	size    int64
}

func (h *CacheCommandHandler) HandleStatus(all bool) error {
	contexts, totalSize, err := h.collectAllContextStatuses(all)
	if err != nil {
		return err
	}

	if len(contexts) == 0 {
		if all {
			output.PrintInfo("No cached data")
		} else {
			name, err := h.configRepository.LoadCurrentContextName()
			if err != nil {
				return err
			}
			output.PrintInfo("No cached data for context '" + name + "'")
		}
		return nil
	}

	output.PrintHeader("Cache")
	output.PrintNewline()

	if all && len(contexts) > 1 {
		output.PrintField("Total size:", formatSize(totalSize))

		for _, ctx := range contexts {
			output.PrintNewline()
			output.PrintLabel(output.Bold(ctx.name) + " " + output.Dim("("+formatSize(ctx.size)+")"))
			for _, entry := range ctx.entries {
				output.PrintBulletField(entry.name, formatSize(entry.size))
			}
		}
	} else {
		ctx := contexts[0]
		output.PrintField("Context:", ctx.name)
		output.PrintField("Path:", ctx.path)
		output.PrintField("Total size:", formatSize(ctx.size))
		output.PrintNewline()

		for _, entry := range ctx.entries {
			output.PrintBulletField(entry.name, formatSize(entry.size))
		}
	}

	return nil
}

func (h *CacheCommandHandler) HandleClear(skipConfirmation bool, all bool) error {
	contexts, totalSize, err := h.collectAllContextStatuses(all)
	if err != nil {
		return err
	}

	if len(contexts) == 0 {
		output.PrintInfo("No cached data to clear")
		return nil
	}

	if !skipConfirmation {
		if !h.terminalInput.IsTerminal() {
			return fmt.Errorf("clearing the cache requires confirmation. Use --yes to skip in non-interactive mode")
		}

		if all {
			output.PrintWarning(fmt.Sprintf("This will remove %s of cached data across %d %s.",
				formatSize(totalSize), len(contexts), output.Plural(len(contexts), "context", "contexts")))
		} else {
			output.PrintWarning(fmt.Sprintf("This will remove %s of cached data for context '%s'.",
				formatSize(totalSize), contexts[0].name))
		}
		for _, ctx := range contexts {
			if all {
				output.PrintWarningLabel(output.Bold(ctx.name) + " " + output.Dim("("+formatSize(ctx.size)+")"))
				for _, entry := range ctx.entries {
					output.PrintWarningBulletField(entry.name, output.Dim("("+formatSize(entry.size)+")"))
				}
			} else {
				for _, entry := range ctx.entries {
					output.PrintWarningDetail(entry.name + " " + output.Dim("("+formatSize(entry.size)+")"))
				}
			}
		}
		output.PrintWarningNewline()

		response, err := h.terminalInput.ReadLine("Continue? [y/N] ")
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if response != "y" && response != "yes" {
			output.PrintInfo("Clear cancelled")
			return nil
		}
		output.PrintNewline()
	}

	output.PrintHeader("Clearing cache")
	output.PrintNewline()

	for i, ctx := range contexts {
		if all {
			if i > 0 {
				output.PrintNewline()
			}
			output.PrintLabel(output.Bold(ctx.name))
		}
		for _, entry := range ctx.entries {
			output.PrintStep("Removing " + entry.name)
			if err := h.fileSystem.RemoveAll(entry.path); err != nil {
				return fmt.Errorf("failed to remove %s: %w", entry.path, err)
			}
		}
	}

	output.PrintNewline()
	output.PrintSuccess(fmt.Sprintf("Cleared %s of cached data", formatSize(totalSize)))

	return nil
}

func (h *CacheCommandHandler) collectAllContextStatuses(all bool) ([]contextStatus, int64, error) {
	contextNames, err := h.resolveContextNames(all)
	if err != nil {
		return nil, 0, err
	}

	var contexts []contextStatus
	var totalSize int64

	for _, name := range contextNames {
		contextPath := filepath.Join("~", ".pilot", name)
		exists, err := h.fileSystem.FileExists(contextPath)
		if err != nil {
			return nil, 0, err
		}
		if !exists {
			continue
		}

		entries, err := h.collectCacheEntries(contextPath)
		if err != nil {
			return nil, 0, err
		}
		if len(entries) == 0 {
			continue
		}

		var size int64
		for _, entry := range entries {
			size += entry.size
		}
		totalSize += size
		contexts = append(contexts, contextStatus{name: name, path: contextPath, entries: entries, size: size})
	}

	return contexts, totalSize, nil
}

func (h *CacheCommandHandler) resolveContextNames(all bool) ([]string, error) {
	if !all {
		name, err := h.configRepository.LoadCurrentContextName()
		if err != nil {
			return nil, err
		}
		return []string{name}, nil
	}

	config, err := h.configRepository.LoadConfig()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(config.Contexts))
	for i, ctx := range config.Contexts {
		names[i] = ctx.Name
	}
	return names, nil
}

func (h *CacheCommandHandler) collectCacheEntries(contextPath string) ([]cacheEntry, error) {
	subdirectories, err := h.fileSystem.ReadSubdirectories(contextPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read context directory: %w", err)
	}

	var entries []cacheEntry
	for _, name := range subdirectories {
		if isStateEntry(name) {
			continue
		}

		entryPath := filepath.Join(contextPath, name)
		size, err := h.fileSystem.DirSize(entryPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate size of %s: %w", entryPath, err)
		}
		if size == 0 {
			continue
		}

		entries = append(entries, cacheEntry{
			name: name,
			path: entryPath,
			size: size,
		})
	}
	return entries, nil
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
