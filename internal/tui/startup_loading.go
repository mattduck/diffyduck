package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// MaxStartupConcurrent is the maximum number of files to load concurrently on startup.
const MaxStartupConcurrent = 8

// initStartupQueue initializes the startup loading queue with all supported files.
// Should be called once on first WindowSizeMsg when we know the model is ready.
func (m *Model) initStartupQueue() tea.Cmd {
	if m.startupQueuedInit {
		return nil
	}
	m.startupQueuedInit = true

	// Can't fetch in pager mode
	if m.pagerMode {
		return nil
	}

	// Queue all supported files that don't already have content
	for i, file := range m.files {
		// Skip files that already have content loaded
		if len(file.NewContent) > 0 || len(file.OldContent) > 0 {
			continue
		}

		// Check if this file is supported by tree-sitter
		filename := file.NewPath
		if filename == "" {
			filename = file.OldPath
		}
		if filename == "" {
			continue
		}

		// Use the filename (not full path) for extension matching
		basename := filepath.Base(filename)
		if m.highlighter != nil && m.highlighter.SupportsFile(basename) {
			m.startupQueue = append(m.startupQueue, i)
		}
	}

	// Start loading up to MaxStartupConcurrent files
	return m.processStartupQueue()
}

// processStartupQueue starts loading files from the startup queue.
// Returns commands for up to MaxStartupConcurrent files.
func (m *Model) processStartupQueue() tea.Cmd {
	if m.fetcher == nil {
		return nil
	}

	var cmds []tea.Cmd

	// Start loading files until we hit the concurrent limit or run out of queue
	for m.startupInFlight < MaxStartupConcurrent && len(m.startupQueue) > 0 {
		// Pop first file from queue
		fileIndex := m.startupQueue[0]
		m.startupQueue = m.startupQueue[1:]

		// Skip if already has content (might have been loaded by another path)
		if fileIndex < len(m.files) {
			file := m.files[fileIndex]
			if len(file.NewContent) > 0 || len(file.OldContent) > 0 {
				continue
			}
		}

		// Start fetching
		if cmd := m.FetchFileContent(fileIndex); cmd != nil {
			m.startupInFlight++
			m.markFileLoading(fileIndex)
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return nil
	}

	// Start spinner if not already ticking
	if cmd := m.startSpinnerIfNeeded(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// onFileLoadComplete should be called when a file finishes loading.
// It decrements the in-flight counter and starts the next file from the queue.
func (m *Model) onStartupFileComplete() tea.Cmd {
	if m.startupInFlight > 0 {
		m.startupInFlight--
	}
	return m.processStartupQueue()
}
