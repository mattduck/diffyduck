package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// PerFileSpinnerDelay is how long a file must be loading before showing its spinner.
// This prevents brief flashes for fast-loading files.
const PerFileSpinnerDelay = 100 * time.Millisecond

// isFileLoading returns true if the given file index is currently loading.
func (m Model) isFileLoading(fileIndex int) bool {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return false
	}
	_, ok := m.loadingFiles[fileIndex]
	return ok
}

// isFileLoadingLongEnough returns true if the file has been loading for at least
// PerFileSpinnerDelay. Used for per-file spinners to avoid brief flashes.
func (m Model) isFileLoadingLongEnough(fileIndex int) bool {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return false
	}
	startTime, ok := m.loadingFiles[fileIndex]
	if !ok {
		return false
	}
	return time.Since(startTime) >= PerFileSpinnerDelay
}

// markFileLoading marks a file as loading.
func (m *Model) markFileLoading(fileIndex int) {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return
	}
	if m.loadingFiles == nil {
		m.loadingFiles = make(map[int]time.Time)
	}
	m.loadingFiles[fileIndex] = time.Now()
}

// clearFileLoading clears the loading state for a file.
func (m *Model) clearFileLoading(fileIndex int) {
	delete(m.loadingFiles, fileIndex)
}

// hasAnyLoadingFiles returns true if any files are currently loading.
func (m Model) hasAnyLoadingFiles() bool {
	return len(m.loadingFiles) > 0 || m.loadingMoreCommits
}

// fileStatusSymbol returns the status symbol for a file, or the spinner frame if loading.
// This replaces the +/-/~/> symbol with a spinner while the file is being fetched or parsed.
// Only shows spinner if file has been loading for PerFileSpinnerDelay.
func (m Model) fileStatusSymbol(fileIndex int, status FileStatus) string {
	if m.isFileLoadingLongEnough(fileIndex) {
		return m.spinner.View()
	}
	symbol, _ := fileStatusIndicator(status)
	return symbol
}

// fileStatusSymbolStyled returns the styled status symbol for a file, or the styled spinner if loading.
// Only shows spinner if file has been loading for PerFileSpinnerDelay.
func (m Model) fileStatusSymbolStyled(fileIndex int, status FileStatus) string {
	if m.isFileLoadingLongEnough(fileIndex) {
		// Style spinner with status color for visual consistency
		_, style := fileStatusIndicator(status)
		return style.Render(m.spinner.View())
	}
	symbol, style := fileStatusIndicator(status)
	return style.Render(symbol)
}

// startSpinnerIfNeeded returns a command to start the spinner if files are loading
// and the spinner isn't already ticking. This prevents multiple tick chains.
func (m *Model) startSpinnerIfNeeded() tea.Cmd {
	if m.hasAnyLoadingFiles() && !m.spinnerTicking {
		m.spinnerTicking = true
		return m.spinner.Tick
	}
	return nil
}

// handleSpinnerTick processes a spinner tick message.
// Returns the updated spinner and a command to continue ticking if files are still loading.
func (m *Model) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)

	// Only continue ticking if files are still loading
	if !m.hasAnyLoadingFiles() {
		m.spinnerTicking = false
		return nil
	}
	return cmd
}
