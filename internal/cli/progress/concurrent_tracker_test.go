package progress

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTracker(names []string, verb string) (*ConcurrentTracker, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	caps := terminalCapabilities{supportsANSI: false, terminalWidth: 80}
	tracker := NewConcurrentTrackerWithWriter(names, verb, buf, false, false, caps)
	return tracker, buf
}

func newTestTrackerWithColor(names []string, verb string) (*ConcurrentTracker, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	caps := terminalCapabilities{supportsANSI: true, terminalWidth: 80}
	tracker := NewConcurrentTrackerWithWriter(names, verb, buf, false, true, caps)
	return tracker, buf
}

func TestConcurrentTracker_CompleteItem_SuccessLifecycle(t *testing.T) {
	tracker, buf := newTestTracker([]string{"service-a"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)
	assert.Equal(t, StatusRunning, tracker.items[0].Status)

	tracker.CompleteItem(0, nil)
	assert.Equal(t, StatusSuccess, tracker.items[0].Status)
	assert.Equal(t, 1, tracker.completed)
	assert.Equal(t, 0, tracker.inProgress)

	tracker.Stop()

	output := buf.String()
	assert.Contains(t, output, "Pulling service-a")
	assert.Contains(t, output, "+ [")
	assert.Contains(t, output, "[1/1]")
	assert.Contains(t, output, "service-a")
	assert.NotContains(t, output, "FAILED")
}

func TestConcurrentTracker_CompleteItem_WithError(t *testing.T) {
	tracker, buf := newTestTracker([]string{"service-a"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, errors.New("connection refused"))

	tracker.Stop()

	assert.Equal(t, StatusFailed, tracker.items[0].Status)
	assert.Equal(t, "connection refused", tracker.items[0].Error.Error())

	output := buf.String()
	assert.Contains(t, output, "x [")
	assert.Contains(t, output, "[1/1]")
	assert.Contains(t, output, "FAILED")
	assert.Contains(t, output, "service-a")
}

func TestConcurrentTracker_CompleteItem_OutOfOrder(t *testing.T) {
	names := []string{"alpha", "beta", "gamma"}
	tracker, buf := newTestTracker(names, "Building")
	tracker.Start()

	// Start all items
	for i := range names {
		tracker.StartItem(i)
	}
	assert.Equal(t, 3, tracker.inProgress)

	// Complete out of order: gamma first, then alpha, then beta (with error)
	tracker.CompleteItem(2, nil)
	assert.Equal(t, 1, tracker.completed)
	assert.Equal(t, 2, tracker.inProgress)

	tracker.CompleteItem(0, nil)
	assert.Equal(t, 2, tracker.completed)
	assert.Equal(t, 1, tracker.inProgress)

	tracker.CompleteItem(1, errors.New("build failed"))
	assert.Equal(t, 3, tracker.completed)
	assert.Equal(t, 0, tracker.inProgress)

	tracker.Stop()

	output := buf.String()

	// Verify counters are sequential
	assert.Contains(t, output, "[1/3]")
	assert.Contains(t, output, "[2/3]")
	assert.Contains(t, output, "[3/3]")

	// Verify completion order: gamma, alpha, beta
	lines := strings.Split(output, "\n")

	// Find completion lines (lines with + or x symbols)
	var completionLines []string
	for _, line := range lines {
		if strings.Contains(line, "+ [") || strings.Contains(line, "x [") {
			completionLines = append(completionLines, line)
		}
	}
	require.Len(t, completionLines, 3)
	assert.Contains(t, completionLines[0], "gamma")
	assert.Contains(t, completionLines[1], "alpha")
	assert.Contains(t, completionLines[2], "beta")

	// Verify FAILED is only on the beta line
	assert.NotContains(t, completionLines[0], "FAILED")
	assert.NotContains(t, completionLines[1], "FAILED")
	assert.Contains(t, completionLines[2], "FAILED")
}

func TestConcurrentTracker_Stop_WithNoItemsStarted(t *testing.T) {
	tracker, _ := newTestTracker([]string{"service-a"}, "Pulling")
	tracker.Start()

	assert.Equal(t, 0, tracker.completed)
	assert.Equal(t, 0, tracker.inProgress)

	// Should not panic
	tracker.Stop()
}

func TestConcurrentTracker_Stop_Idempotent(t *testing.T) {
	tracker, _ := newTestTracker([]string{"service-a"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, nil)

	// Multiple Stop calls should not panic
	require.NotPanics(t, func() {
		tracker.Stop()
		tracker.Stop()
		tracker.Stop()
	})
}

func TestConcurrentTracker_CompleteItem_SequentialCounters(t *testing.T) {
	names := []string{"svc-1", "svc-2"}
	tracker, buf := newTestTracker(names, "Installing")
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, nil)

	tracker.StartItem(1)
	tracker.CompleteItem(1, nil)

	tracker.Stop()

	output := buf.String()
	assert.Contains(t, output, "[1/2]")
	assert.Contains(t, output, "[2/2]")
	assert.NotContains(t, output, "FAILED")
}

func TestConcurrentTracker_StartItem_NonTTYOutput(t *testing.T) {
	tracker, buf := newTestTracker([]string{"my-service"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)

	output := buf.String()
	// Counter shows [completed/total] at time of start, so 0 completed out of 1
	assert.Contains(t, output, "[0/1]")
	assert.Contains(t, output, "Pulling my-service")
	assert.Contains(t, output, "(1 in progress)")

	tracker.CompleteItem(0, nil)
	tracker.Stop()
}

func TestConcurrentTracker_CompleteItem_IncludesDuration(t *testing.T) {
	tracker, buf := newTestTracker([]string{"service-a"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, nil)
	tracker.Stop()

	output := buf.String()
	assert.Contains(t, output, "(<1s)")
}

func TestConcurrentTracker_CompleteItem_SuccessWithColor(t *testing.T) {
	tracker, buf := newTestTrackerWithColor([]string{"service-a"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, nil)
	tracker.Stop()

	output := buf.String()
	assert.Contains(t, output, "\033[32m+\033[0m") // green success symbol
	assert.Contains(t, output, "\033[2m")          // dim formatting
	assert.NotContains(t, output, "FAILED")
}

func TestConcurrentTracker_CompleteItem_FailureWithColor(t *testing.T) {
	tracker, buf := newTestTrackerWithColor([]string{"service-a"}, "Pulling")
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, errors.New("timeout"))
	tracker.Stop()

	output := buf.String()
	assert.Contains(t, output, "\033[31mx\033[0m") // red failure symbol
	assert.Contains(t, output, "FAILED")
}

func TestConcurrentTracker_Stop_ZeroItems(t *testing.T) {
	tracker, _ := newTestTracker([]string{}, "Pulling")
	tracker.Start()

	assert.Equal(t, 0, tracker.total)
	assert.Equal(t, 0, tracker.completed)

	// Should not panic
	require.NotPanics(t, func() {
		tracker.Stop()
	})
}

func TestConcurrentTracker_Start_TTYStartsAndStopsCleanly(t *testing.T) {
	buf := &bytes.Buffer{}
	caps := terminalCapabilities{supportsANSI: false, terminalWidth: 80}
	tracker := NewConcurrentTrackerWithWriter([]string{"svc"}, "Pulling", buf, true, false, caps)
	tracker.Start()

	tracker.StartItem(0)
	tracker.CompleteItem(0, nil)

	require.NotPanics(t, func() {
		tracker.Stop()
	})
}
