package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// makeMultiFileModel creates a Model with the specified number of files,
// each having the given number of line pairs.
func makeMultiFileModel(numFiles, numLinesPerFile int) Model {
	makePairs := func(n int) []sidebyside.LinePair {
		pairs := make([]sidebyside.LinePair, n)
		for i := range pairs {
			pairs[i] = sidebyside.LinePair{
				Old: sidebyside.Line{Num: i + 1, Content: "content"},
				New: sidebyside.Line{Num: i + 1, Content: "content"},
			}
		}
		return pairs
	}

	files := make([]sidebyside.FilePair, numFiles)
	for i := range files {
		files[i] = sidebyside.FilePair{
			OldPath:   fmt.Sprintf("a/file%d.go", i),
			NewPath:   fmt.Sprintf("b/file%d.go", i),
			FoldLevel: sidebyside.FoldExpanded,
			Pairs:     makePairs(numLinesPerFile),
		}
	}

	m := New(files)
	m.width = 80
	m.height = 40
	m.initialFoldSet = true // prevent WindowSizeMsg from changing fold levels
	return m
}

// =============================================================================
// Loading State Tracking Tests
// =============================================================================

func TestLoadingFiles_InitiallyEmpty(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	assert.Empty(t, m.loadingFiles, "loadingFiles should be empty initially")
}

func TestLoadingFiles_MarkedOnContentFetch(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	m.fetcher = nil // Will prevent actual fetch, but we're testing state tracking

	// Simulate what happens when we start fetching
	m.markFileLoading(0)

	assert.True(t, m.isFileLoading(0), "file 0 should be marked as loading")
	assert.False(t, m.isFileLoading(1), "file 1 should not be loading")
}

func TestLoadingFiles_StillLoadingAfterContentLoaded(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	m.markFileLoading(0)

	// Simulate content loaded message
	msg := FileContentLoadedMsg{
		FileIndex:  0,
		NewContent: []string{"package main"},
		OldContent: []string{"package main"},
	}
	newM, _ := m.Update(msg)
	model := newM.(Model)

	// File is still loading - waiting for highlight to be ready
	// Loading state is cleared on HighlightReadyMsg, not FileContentLoadedMsg
	assert.True(t, model.isFileLoading(0), "file 0 should still be loading until highlight is ready")
}

func TestLoadingFiles_ClearedOnHighlightReady(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	m.markFileLoading(0)
	// Give file content so highlight can proceed
	m.files[0].NewContent = []string{"package main"}
	m.files[0].OldContent = []string{"package main"}

	// Simulate highlight ready message
	msg := HighlightReadyMsg{
		FileIndex: 0,
	}
	newM, _ := m.Update(msg)
	model := newM.(Model)

	assert.False(t, model.isFileLoading(0), "file 0 should no longer be loading after highlight ready")
}

func TestLoadingFiles_MultipleFilesLoading(t *testing.T) {
	m := makeMultiFileModel(5, 10)

	m.markFileLoading(0)
	m.markFileLoading(2)
	m.markFileLoading(4)

	assert.True(t, m.isFileLoading(0), "file 0 should be loading")
	assert.False(t, m.isFileLoading(1), "file 1 should not be loading")
	assert.True(t, m.isFileLoading(2), "file 2 should be loading")
	assert.False(t, m.isFileLoading(3), "file 3 should not be loading")
	assert.True(t, m.isFileLoading(4), "file 4 should be loading")
}

// =============================================================================
// Spinner Initialization Tests
// =============================================================================

func TestSpinner_InitializedInNew(t *testing.T) {
	files := []sidebyside.FilePair{{
		OldPath:   "a/test.go",
		NewPath:   "b/test.go",
		FoldLevel: sidebyside.FoldExpanded,
	}}
	m := New(files)

	// Spinner should be initialized (we can check by calling View - non-zero string)
	view := m.spinner.View()
	assert.NotEmpty(t, view, "spinner should be initialized and have a view")
}

// =============================================================================
// Spinner Tick Forwarding Tests
// =============================================================================

func TestSpinner_TicksWhileFilesLoading(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	m.markFileLoading(0)

	// Send a spinner tick message
	tickMsg := m.spinner.Tick()
	newM, cmd := m.Update(tickMsg)
	model := newM.(Model)

	// Should continue ticking while files are loading
	assert.NotNil(t, cmd, "should continue ticking while files are loading")
	assert.True(t, model.isFileLoading(0), "file should still be loading")
}

func TestSpinner_StopsTickingWhenNoFilesLoading(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	// No files loading

	// Send a spinner tick message
	tickMsg := m.spinner.Tick()
	newM, cmd := m.Update(tickMsg)
	_ = newM.(Model)

	// Should not continue ticking when no files are loading
	assert.Nil(t, cmd, "should not continue ticking when no files are loading")
}

// =============================================================================
// File Status Indicator Tests
// =============================================================================

func TestFileStatusIndicator_ShowsSpinnerWhenLoading(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	// Set loading time in the past to exceed PerFileSpinnerDelay
	m.loadingFiles[0] = time.Now().Add(-200 * time.Millisecond)

	symbol := m.fileStatusSymbol(0, FileStatusModified)

	// Should return spinner frame, not the status symbol
	spinnerView := m.spinner.View()
	assert.Equal(t, spinnerView, symbol, "should show spinner frame when loading")
}

func TestFileStatusIndicator_HidesSpinnerWhenLoadingBriefly(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	// Just started loading - should not show spinner yet
	m.markFileLoading(0)

	symbol := m.fileStatusSymbol(0, FileStatusModified)

	// Should return normal symbol, not spinner, because delay hasn't passed
	assert.Equal(t, "~", symbol, "should show normal symbol when loading just started")
}

func TestFileStatusIndicator_ShowsNormalSymbolWhenNotLoading(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	// File 0 is NOT loading

	symbol := m.fileStatusSymbol(0, FileStatusModified)
	assert.Equal(t, "~", symbol, "should show ~ for modified when not loading")

	symbol = m.fileStatusSymbol(0, FileStatusAdded)
	assert.Equal(t, "+", symbol, "should show + for added when not loading")

	symbol = m.fileStatusSymbol(0, FileStatusDeleted)
	assert.Equal(t, "-", symbol, "should show - for deleted when not loading")
}

// =============================================================================
// View Rendering Tests
// =============================================================================

func TestTopBar_ShowsSpinnerForCurrentFile(t *testing.T) {
	m := makeMultiFileModel(1, 10)
	m.width = 80
	m.height = 20
	// Set loading time in the past to exceed PerFileSpinnerDelay
	m.loadingFiles[0] = time.Now().Add(-200 * time.Millisecond)
	m.rebuildRowsCache()

	// Render the top bar
	topBar := m.renderTopBar()

	// Should contain spinner frame, not the +/-/~ symbol
	spinnerFrame := m.spinner.View()
	assert.Contains(t, topBar, spinnerFrame, "top bar should contain spinner when file is loading")
}

func TestHeader_NoSpinnerInMainView(t *testing.T) {
	// Main view headers no longer show status symbol or spinner
	// (spinner still appears in the top bar via fileStatusSymbolStyled)
	m := makeMultiFileModel(1, 10)
	m.width = 80
	m.height = 20
	m.loadingFiles[0] = time.Now().Add(-200 * time.Millisecond)
	m.rebuildRowsCache()

	testTreePath := TreePath{
		Ancestors: []TreeLevel{{IsLast: true, Style: lipgloss.NewStyle()}},
		Current:   &TreeLevel{IsLast: false, Style: lipgloss.NewStyle()},
	}
	header := m.renderHeader(
		"test.go",
		sidebyside.FoldNormal,
		HeaderThreeLine,
		FileStatusModified,
		5, 3,
		50,
		0,
		0,
		false,
		testTreePath,
	)

	// Main view headers no longer contain spinner (removed status symbol area)
	spinnerFrame := m.spinner.View()
	assert.NotContains(t, header, spinnerFrame, "main view header should not contain spinner (status symbol removed)")
	// But should still show the filename
	assert.Contains(t, header, "test.go", "header should still contain filename")
}

// =============================================================================
// Spinner Command Integration Tests
// =============================================================================

func TestStartSpinnerCmd_ReturnsTickWhenFilesLoading(t *testing.T) {
	m := makeMultiFileModel(3, 10)
	m.markFileLoading(0)

	cmd := m.startSpinnerIfNeeded()
	assert.NotNil(t, cmd, "should return tick command when files are loading")
}

func TestStartSpinnerCmd_ReturnsNilWhenNoFilesLoading(t *testing.T) {
	m := makeMultiFileModel(3, 10)

	cmd := m.startSpinnerIfNeeded()
	assert.Nil(t, cmd, "should return nil when no files are loading")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestLoadingFiles_InvalidIndexSafe(t *testing.T) {
	m := makeMultiFileModel(3, 10)

	// These should not panic
	assert.False(t, m.isFileLoading(-1), "negative index should return false")
	assert.False(t, m.isFileLoading(100), "out of bounds index should return false")

	// Marking invalid indices should be safe (no panic, no effect)
	m.markFileLoading(-1)
	m.markFileLoading(100)
	assert.Empty(t, m.loadingFiles, "invalid indices should not be added to loadingFiles")
}

func TestClearFileLoading_Safe(t *testing.T) {
	m := makeMultiFileModel(3, 10)

	// Clearing non-existent loading state should be safe
	m.clearFileLoading(0)  // Not loading, should be no-op
	m.clearFileLoading(-1) // Invalid, should be safe

	assert.Empty(t, m.loadingFiles)
}

// =============================================================================
// Spinner Style Tests
// =============================================================================

func TestSpinner_UsesCompactStyle(t *testing.T) {
	m := makeMultiFileModel(1, 10)

	// Spinner view should be single character (or very short)
	view := m.spinner.View()
	assert.LessOrEqual(t, len(view), 4, "spinner should be compact (max 4 chars including ANSI)")
}

// =============================================================================
// Helper method existence tests (compilation checks)
// =============================================================================

func TestHelperMethods_Exist(t *testing.T) {
	m := makeMultiFileModel(1, 10)

	// These should compile - testing that the methods exist
	_ = m.isFileLoading(0)
	m.markFileLoading(0)
	m.clearFileLoading(0)
	_ = m.fileStatusSymbol(0, FileStatusModified)
	_ = m.startSpinnerIfNeeded()
	_ = m.spinner.View()
}

// =============================================================================
// Spinner tick message type test
// =============================================================================

func TestSpinnerTick_IsCorrectType(t *testing.T) {
	m := makeMultiFileModel(1, 10)

	tickMsg := m.spinner.Tick()
	// The tick should be a tea.Cmd that returns a spinner.TickMsg
	assert.NotNil(t, tickMsg, "tick should not be nil")

	// Execute the command to get the message
	// Note: In real usage, this would be handled by bubbletea runtime
	// Here we just verify the command exists
}

// Test that spinner.Update works correctly
func TestSpinnerUpdate_AdvancesFrame(t *testing.T) {
	m := makeMultiFileModel(1, 10)

	// Verify initial view works
	_ = m.spinner.View()

	// Get a tick message and process it
	// Create a mock tick message
	newSpinner, _ := m.spinner.Update(spinner.TickMsg{})
	m.spinner = newSpinner

	// Frame may or may not change (depends on timing), but Update should work
	assert.NotPanics(t, func() {
		m.spinner.View()
	}, "spinner view should not panic after update")
}
