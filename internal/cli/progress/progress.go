package progress

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// Status represents the state of a task
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusSuccess
	StatusFailed
)

// Item represents a single task being tracked
type Item struct {
	Name     string
	Info     string // Additional info (e.g., repo, ref)
	Status   Status
	Duration time.Duration
	Error    error
}

// Tracker manages progress display for multiple sequential tasks
type Tracker struct {
	mu           sync.Mutex
	wg           sync.WaitGroup
	items        []Item
	current      int
	total        int
	startTime    time.Time
	isTTY        bool
	useColor     bool
	caps         terminalCapabilities
	stopChan     chan struct{}
	stopOnce     sync.Once
	spinnerFrame int
	actionVerb   string // e.g., "Building", "Installing", "Pulling"
}

var spinnerFrames = []string{"✦", "✸", "✹", "❋", "✹", "✸"}

// NewTracker creates a new progress tracker with names only
func NewTracker(names []string) *Tracker {
	return NewTrackerWithVerb(names, "Processing")
}

// NewTrackerWithVerb creates a new progress tracker with a custom action verb
func NewTrackerWithVerb(names []string, verb string) *Tracker {
	items := make([]Item, len(names))
	for i, name := range names {
		items[i] = Item{Name: name, Status: StatusPending}
	}

	_, noColor := os.LookupEnv("NO_COLOR")
	isTTY := term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // safe fd conversion
	caps := detectCapabilities()

	return &Tracker{
		items:      items,
		current:    -1,
		total:      len(names),
		isTTY:      isTTY,
		useColor:   !noColor && isTTY && caps.supportsANSI,
		caps:       caps,
		stopChan:   make(chan struct{}),
		actionVerb: verb,
	}
}

// NewTrackerWithInfo creates a new progress tracker with names and additional info
func NewTrackerWithInfo(names []string, infos []string) *Tracker {
	return NewTrackerWithInfoAndVerb(names, infos, "Processing")
}

// NewTrackerWithInfoAndVerb creates a new progress tracker with names, info, and custom verb
func NewTrackerWithInfoAndVerb(names []string, infos []string, verb string) *Tracker {
	items := make([]Item, len(names))
	for i, name := range names {
		info := ""
		if i < len(infos) {
			info = infos[i]
		}
		items[i] = Item{Name: name, Info: info, Status: StatusPending}
	}

	_, noColor := os.LookupEnv("NO_COLOR")
	isTTY := term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // safe fd conversion
	caps := detectCapabilities()

	return &Tracker{
		items:      items,
		current:    -1,
		total:      len(names),
		isTTY:      isTTY,
		useColor:   !noColor && isTTY && caps.supportsANSI,
		caps:       caps,
		stopChan:   make(chan struct{}),
		actionVerb: verb,
	}
}

// Start begins tracking and starts the spinner animation if in TTY mode
func (t *Tracker) Start() {
	if t.isTTY {
		t.wg.Add(1)
		go t.animate()
	}
}

// StartItem marks an item as running
func (t *Tracker) StartItem(index int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.current = index
	t.items[index].Status = StatusRunning
	t.startTime = time.Now()

	if !t.isTTY {
		// Non-TTY mode: print timestamped start message
		ts := time.Now().Format("15:04:05")
		item := t.items[index]
		counter := fmt.Sprintf("[%d/%d]", index+1, t.total)
		if item.Info != "" {
			fmt.Printf("[%s] %s %s %s (%s)...\n", ts, counter, t.actionVerb, item.Name, item.Info)
		} else {
			fmt.Printf("[%s] %s %s %s...\n", ts, counter, t.actionVerb, item.Name)
		}
	}
}

// CompleteItem marks an item as completed (success or failure)
func (t *Tracker) CompleteItem(index int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.items[index].Duration = time.Since(t.startTime)

	if err != nil {
		t.items[index].Status = StatusFailed
		t.items[index].Error = err
	} else {
		t.items[index].Status = StatusSuccess
	}

	if !t.isTTY {
		// Non-TTY mode: print timestamped completion
		ts := time.Now().Format("15:04:05")
		sym := "+"
		status := "completed"
		if err != nil {
			sym = "x"
			status = "FAILED"
		}
		fmt.Printf(
			"[%s] %s %s %s (%s)\n",
			ts,
			sym,
			t.items[index].Name,
			status,
			FormatDuration(t.items[index].Duration),
		)
		// Note: Error details are printed by the root error handler, not here
	}
}

// Stop ends the progress tracking
func (t *Tracker) Stop() {
	t.stopOnce.Do(
		func() {
			close(t.stopChan)
		},
	)

	// Wait for animate goroutine to finish
	t.wg.Wait()

	if t.isTTY {
		t.mu.Lock()
		if t.useColor {
			fmt.Print("\033[0m") // Ensure terminal state is reset
		}
		t.printFinal()
		t.mu.Unlock()
	}
}

// GetStatus returns a formatted status line for the current item (for TTY mode)
func (t *Tracker) GetStatus() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.current < 0 || t.current >= len(t.items) {
		return ""
	}

	item := t.items[t.current]
	if item.Status != StatusRunning {
		return ""
	}

	elapsed := time.Since(t.startTime)
	spinner := spinnerFrames[t.spinnerFrame%len(spinnerFrames)]
	counter := fmt.Sprintf("[%d/%d]", t.current+1, t.total)

	var line string
	if t.useColor {
		if item.Info != "" {
			line = fmt.Sprintf(
				"  \033[1m%s %s  %s\033[0m  \033[2m(%s)  %s\033[0m",
				spinner,
				counter,
				item.Name,
				item.Info,
				FormatDuration(elapsed),
			)
		} else {
			line = fmt.Sprintf(
				"  \033[1m%s %s  %s\033[0m  \033[2m%s\033[0m",
				spinner,
				counter,
				item.Name,
				FormatDuration(elapsed),
			)
		}
	} else {
		displayName := t.formatDisplayName(item)
		line = fmt.Sprintf("  %s %s  %s  %s", spinner, counter, displayName, FormatDuration(elapsed))
	}

	// Truncate to terminal width to prevent line wrapping
	return clearLine(t.caps) + truncateToWidth(line, t.caps.terminalWidth)
}

func (t *Tracker) animate() {
	defer t.wg.Done()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.mu.Lock()
			if t.current >= 0 && t.items[t.current].Status == StatusRunning {
				t.spinnerFrame++
				elapsed := time.Since(t.startTime)
				spinner := spinnerFrames[t.spinnerFrame%len(spinnerFrames)]

				// Clear line and print status
				item := t.items[t.current]
				counter := fmt.Sprintf("[%d/%d]", t.current+1, t.total)

				var line string
				if t.useColor {
					if item.Info != "" {
						line = fmt.Sprintf(
							"  \033[1m%s %s  %s\033[0m  \033[2m(%s)  %s\033[0m",
							spinner,
							counter,
							item.Name,
							item.Info,
							FormatDuration(elapsed),
						)
					} else {
						line = fmt.Sprintf(
							"  \033[1m%s %s  %s\033[0m  \033[2m%s\033[0m",
							spinner,
							counter,
							item.Name,
							FormatDuration(elapsed),
						)
					}
				} else {
					displayName := t.formatDisplayName(item)
					line = fmt.Sprintf("  %s %s  %s  %s", spinner, counter, displayName, FormatDuration(elapsed))
				}

				// Truncate to terminal width to prevent line wrapping
				line = truncateToWidth(line, t.caps.terminalWidth)
				fmt.Print(clearLine(t.caps) + line)
			}
			t.mu.Unlock()
		}
	}
}

func (t *Tracker) printFinal() {
	// Clear current line
	fmt.Print(clearLine(t.caps))
}

// formatDisplayName formats the item name with optional info
func (t *Tracker) formatDisplayName(item Item) string {
	if item.Info == "" {
		return item.Name
	}
	if t.useColor {
		return fmt.Sprintf("%s \033[2m(%s)\033[0m", item.Name, item.Info)
	}
	return fmt.Sprintf("%s (%s)", item.Name, item.Info)
}

// PrintItemStart prints the start of an item (used with TTY for the status line)
func (t *Tracker) PrintItemStart(index int, prefix string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isTTY {
		// In TTY mode, we'll update in place
		fmt.Print(prefix)
	}
}

// PrintItemComplete prints the completion status of an item
func (t *Tracker) PrintItemComplete(index int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isTTY {
		return // Already printed in CompleteItem for non-TTY
	}

	item := t.items[index]

	// Clear the spinner line and move to new line
	fmt.Print(clearLine(t.caps))

	var sym string
	var suffix string

	switch item.Status {
	case StatusSuccess:
		if t.useColor {
			sym = "\033[32m+\033[0m" // green
		} else {
			sym = "+"
		}
		suffix = fmt.Sprintf("(%s)", FormatDuration(item.Duration))
	case StatusFailed:
		if t.useColor {
			sym = "\033[31mx\033[0m" // red
		} else {
			sym = "x"
		}
		suffix = fmt.Sprintf("(%s) FAILED", FormatDuration(item.Duration))
	}

	counter := fmt.Sprintf("[%d/%d]", index+1, t.total)
	displayName := t.formatDisplayName(item)

	// Dim the counter and duration
	if t.useColor {
		counter = fmt.Sprintf("\033[2m%s\033[0m", counter)
		suffix = fmt.Sprintf("\033[2m%s\033[0m", suffix)
	}

	fmt.Printf("  %s %s  %s  %s\n", sym, counter, displayName, suffix)
	// Note: Error details are printed by the root error handler, not here
}

// FormatDuration formats a duration as a human-readable string (e.g., "12m 43s", "5s", or "<1s")
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	// Truncate to seconds so each display state lasts a full second
	totalSeconds := int(d.Seconds())
	m := totalSeconds / 60
	s := totalSeconds % 60

	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// Summary returns a summary string of completed tasks
func (t *Tracker) Summary() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var totalDuration time.Duration
	successCount := 0
	failCount := 0

	for _, item := range t.items {
		totalDuration += item.Duration
		switch item.Status {
		case StatusSuccess:
			successCount++
		case StatusFailed:
			failCount++
		}
	}

	var parts []string
	if successCount > 0 {
		parts = append(parts, fmt.Sprintf("%d succeeded", successCount))
	}
	if failCount > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failCount))
	}

	return fmt.Sprintf("%s in %s", strings.Join(parts, ", "), FormatDuration(totalDuration))
}
