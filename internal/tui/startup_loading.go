package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// MaxStartupConcurrent is the maximum number of files to load concurrently on startup.
const MaxStartupConcurrent = 8

// initStartupQueue initializes the startup loading queue with supported files.
// Only queues files in non-folded commits to avoid unnecessary work.
// Should be called once on first WindowSizeMsg when we know the model is ready.
func (m *Model) initStartupQueue() tea.Cmd {
	if m.startupQueuedInit {
		return nil
	}
	m.startupQueuedInit = true

	// Queue supported files for preloading.
	// In diff/show mode (fetcher set), preload everything — the user will expand the commit.
	// In log mode (no fetcher), skip files in folded commits with metadata (they'll be
	// loaded on demand when expanded), but not for no-metadata commits where
	// CommitFolded doesn't hide files.
	for i, file := range m.files {
		if m.fetcher == nil {
			commitIdx := m.commitForFile(i)
			if commitIdx >= 0 && commitIdx < len(m.commits) {
				if m.commitFoldLevel(commitIdx) == sidebyside.CommitFolded &&
					m.commits[commitIdx].Info.HasMetadata() {
					continue
				}
			}
		}

		if m.shouldQueueFile(file) {
			m.startupQueue = append(m.startupQueue, i)
		}
	}

	// Start loading up to MaxStartupConcurrent files
	return m.processStartupQueue()
}

// shouldQueueFile returns true if the file should be queued for loading.
func (m *Model) shouldQueueFile(file sidebyside.FilePair) bool {
	// Skip files that already have content loaded
	if len(file.NewContent) > 0 || len(file.OldContent) > 0 {
		return false
	}

	// Check if this file is supported by tree-sitter
	filename := file.NewPath
	if filename == "" {
		filename = file.OldPath
	}
	if filename == "" {
		return false
	}

	// Use the filename (not full path) for extension/filename matching.
	// Note: this can't detect shebang-only files since content isn't loaded yet.
	// Those files will still get highlighted on demand when expanded.
	basename := filepath.Base(filename)
	return m.highlighter != nil && m.highlighter.SupportsFile(basename)
}

// queueFilesForAllCommits queues files for all non-folded commits.
// Called when shift-tab expands all commits at once.
func (m *Model) queueFilesForAllCommits() tea.Cmd {
	for i := range m.commits {
		if m.commitFoldLevel(i) != sidebyside.CommitFolded {
			// Get file range for this commit
			startIdx := m.commitFileStarts[i]
			endIdx := len(m.files)
			if i+1 < len(m.commits) {
				endIdx = m.commitFileStarts[i+1]
			}

			// Queue supported files that don't already have content
			for j := startIdx; j < endIdx; j++ {
				if m.shouldQueueFile(m.files[j]) {
					m.startupQueue = append(m.startupQueue, j)
				}
			}
		}
	}

	return m.processStartupQueue()
}

// processStartupQueue starts loading files from the startup queue.
// Returns commands for up to MaxStartupConcurrent files.
func (m *Model) processStartupQueue() tea.Cmd {
	// Need either a fetcher (diff/show) or git object (log mode, creates on-demand fetchers)
	if m.fetcher == nil && m.git == nil {
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
