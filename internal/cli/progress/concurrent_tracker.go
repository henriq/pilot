package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// ConcurrentTrackerItem represents a single task being tracked concurrently
type ConcurrentTrackerItem struct {
	Name      string
	Status    Status
	Duration  time.Duration
	Error     error
	startTime time.Time
}

// ConcurrentTracker manages progress display for multiple concurrent tasks
type ConcurrentTracker struct {
	mu           sync.Mutex
	wg           sync.WaitGroup
	items        []ConcurrentTrackerItem
	total        int
	completed    int
	inProgress   int
	isTTY        bool
	useColor     bool
	caps         terminalCapabilities
	stopChan     chan struct{}
	stopOnce     sync.Once
	spinnerFrame int
	actionVerb   string
	startTime    time.Time
	writer       io.Writer
}

// NewConcurrentTracker creates a new concurrent progress tracker
func NewConcurrentTracker(names []string, verb string) *ConcurrentTracker {
	items := make([]ConcurrentTrackerItem, len(names))
	for i, name := range names {
		items[i] = ConcurrentTrackerItem{Name: name, Status: StatusPending}
	}

	_, noColor := os.LookupEnv("NO_COLOR")
	isTTY := term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // safe fd conversion
	caps := detectCapabilities()

	return &ConcurrentTracker{
		items:      items,
		total:      len(names),
		isTTY:      isTTY,
		useColor:   !noColor && isTTY && caps.supportsANSI,
		caps:       caps,
		stopChan:   make(chan struct{}),
		actionVerb: verb,
		writer:     os.Stdout,
	}
}

// NewConcurrentTrackerWithWriter creates a concurrent tracker with an injectable writer
// and explicit terminal settings, bypassing auto-detection. Intended for testing.
func NewConcurrentTrackerWithWriter(names []string, verb string, writer io.Writer, isTTY bool, useColor bool, caps terminalCapabilities) *ConcurrentTracker {
	items := make([]ConcurrentTrackerItem, len(names))
	for i, name := range names {
		items[i] = ConcurrentTrackerItem{Name: name, Status: StatusPending}
	}

	return &ConcurrentTracker{
		items:      items,
		total:      len(names),
		isTTY:      isTTY,
		useColor:   useColor,
		caps:       caps,
		stopChan:   make(chan struct{}),
		actionVerb: verb,
		writer:     writer,
	}
}

// Start begins tracking and starts the spinner animation if in TTY mode
func (t *ConcurrentTracker) Start() {
	t.startTime = time.Now()
	if t.isTTY {
		t.wg.Add(1)
		go t.animate()
	}
}

// StartItem marks an item as running with its own start time
func (t *ConcurrentTracker) StartItem(index int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.items[index].Status = StatusRunning
	t.items[index].startTime = time.Now()
	t.inProgress++

	if !t.isTTY {
		// Non-TTY mode: print timestamped start message
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(t.writer, "[%s] [%d/%d] %s %s (%d in progress)...\n",
			ts, t.completed, t.total, t.actionVerb, t.items[index].Name, t.inProgress)
	}
}

// CompleteItem marks an item as completed (success or failure) and prints its status
func (t *ConcurrentTracker) CompleteItem(index int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.items[index].Duration = time.Since(t.items[index].startTime)

	if err != nil {
		t.items[index].Status = StatusFailed
		t.items[index].Error = err
	} else {
		t.items[index].Status = StatusSuccess
	}

	t.inProgress--
	t.completed++

	// Print completion status
	item := t.items[index]

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

	counter := fmt.Sprintf("[%d/%d]", t.completed, t.total)

	if t.isTTY {
		// Clear the spinner line first
		fmt.Fprint(t.writer, clearLine(t.caps))
	} else {
		// Non-TTY mode: add timestamp
		ts := time.Now().Format("15:04:05")
		counter = fmt.Sprintf("[%s] %s", ts, counter)
	}

	// Dim the counter and duration
	if t.useColor {
		counter = fmt.Sprintf("\033[2m%s\033[0m", counter)
		suffix = fmt.Sprintf("\033[2m%s\033[0m", suffix)
	}

	fmt.Fprintf(t.writer, "  %s %s  %s  %s\n", sym, counter, item.Name, suffix)
}

// Stop ends the progress tracking
func (t *ConcurrentTracker) Stop() {
	t.stopOnce.Do(func() {
		close(t.stopChan)
	})

	// Wait for animate goroutine to finish
	t.wg.Wait()

	if t.isTTY {
		t.mu.Lock()
		if t.useColor {
			fmt.Fprint(t.writer, "\033[0m") // Ensure terminal state is reset
		}
		// Clear any remaining spinner line
		fmt.Fprint(t.writer, clearLine(t.caps))
		t.mu.Unlock()
	}
}

func (t *ConcurrentTracker) animate() {
	defer t.wg.Done()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.mu.Lock()
			if t.inProgress > 0 {
				t.spinnerFrame++
				elapsed := time.Since(t.startTime)
				spinner := spinnerFrames[t.spinnerFrame%len(spinnerFrames)]

				var line string
				counter := fmt.Sprintf("[%d/%d]", t.completed, t.total)
				status := fmt.Sprintf("%d in progress...", t.inProgress)

				if t.useColor {
					line = fmt.Sprintf(
						"  \033[1m%s %s  %s\033[0m  \033[2m%s\033[0m",
						spinner,
						counter,
						status,
						FormatDuration(elapsed),
					)
				} else {
					line = fmt.Sprintf("  %s %s  %s  %s", spinner, counter, status, FormatDuration(elapsed))
				}

				// Truncate to terminal width to prevent line wrapping
				line = truncateToWidth(line, t.caps.terminalWidth)
				fmt.Fprint(t.writer, clearLine(t.caps)+line)
			}
			t.mu.Unlock()
		}
	}
}
