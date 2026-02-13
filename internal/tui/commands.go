package tui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/content"
)

// fetcherForFile returns a fetcher suitable for the given file index.
// Uses the persistent fetcher (diff/show mode) or creates one from the commit's SHA.
func (m Model) fetcherForFile(fileIndex int) *content.Fetcher {
	if m.fetcher != nil {
		return m.fetcher
	}
	if m.git != nil && len(m.commits) > 0 {
		commitIdx := m.commitForFile(fileIndex)
		if commitIdx >= 0 && commitIdx < len(m.commits) {
			commit := m.commits[commitIdx]
			if commit.IsSnapshot && commit.SnapshotOldRef != "" && commit.SnapshotNewRef != "" {
				return content.NewFetcher(m.git, content.ModeDiffRefs, commit.SnapshotOldRef, commit.SnapshotNewRef)
			} else if commit.Info.SHA != "" {
				return content.NewFetcher(m.git, content.ModeShow, commit.Info.SHA, "")
			}
		}
	}
	return nil
}

// loadFileContentSync fetches content for a file synchronously and stores it on the model.
// Returns true if content is available (already loaded or successfully fetched).
func (m *Model) loadFileContentSync(fileIndex int) bool {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return false
	}
	fp := &m.files[fileIndex]
	if fp.HasContent() {
		return true
	}
	if fp.IsBinary {
		return false
	}

	fetcher := m.fetcherForFile(fileIndex)
	if fetcher == nil {
		return false
	}

	oldPath := stripPathPrefix(fp.OldPath)
	newPath := stripPathPrefix(fp.NewPath)

	if fp.OldPath != "/dev/null" {
		lines, wasTruncated, err := fetcher.GetOldContentLines(oldPath)
		if err == nil {
			fp.OldContent = lines
			fp.OldContentTruncated = wasTruncated
		}
	}
	if fp.NewPath != "/dev/null" {
		lines, wasTruncated, err := fetcher.GetNewContentLines(newPath)
		if err == nil {
			fp.NewContent = lines
			fp.NewContentTruncated = wasTruncated
		}
	}
	fp.ContentTruncated = fp.OldContentTruncated || fp.NewContentTruncated
	return true
}

// loadAndHighlightFileSync loads content and highlights a file synchronously.
// Used when expanding files/commits to ensure highlighting is ready before render.
func (m *Model) loadAndHighlightFileSync(fileIndex int) {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return
	}
	// Skip if already highlighted
	if m.highlightSpans[fileIndex] != nil {
		return
	}
	m.loadFileContentSync(fileIndex)
	m.highlightFileSync(fileIndex)
}

// FetchFileContent returns a command that fetches content for one file.
// Content is fetched with limits applied (max lines, max line length, max bytes).
// Returns nil for binary files since they have no viewable text content.
func (m Model) FetchFileContent(fileIndex int) tea.Cmd {
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return nil
	}

	fp := m.files[fileIndex]

	// Skip fetching content for binary files
	if fp.IsBinary {
		return nil
	}

	fetcher := m.fetcherForFile(fileIndex)
	if fetcher == nil {
		return nil
	}

	return func() tea.Msg {
		oldPath := stripPathPrefix(fp.OldPath)
		newPath := stripPathPrefix(fp.NewPath)

		var oldContent, newContent []string
		var oldTruncated, newTruncated bool

		// Fetch old content (unless it's a new file)
		if fp.OldPath != "/dev/null" {
			lines, wasTruncated, err := fetcher.GetOldContentLines(oldPath)
			if err == nil {
				oldContent = lines
				oldTruncated = wasTruncated
			}
		}

		// Fetch new content (unless it's a deleted file)
		if fp.NewPath != "/dev/null" {
			lines, wasTruncated, err := fetcher.GetNewContentLines(newPath)
			if err == nil {
				newContent = lines
				newTruncated = wasTruncated
			}
		}

		return FileContentLoadedMsg{
			FileIndex:        fileIndex,
			OldContent:       oldContent,
			NewContent:       newContent,
			ContentTruncated: oldTruncated || newTruncated, // legacy field
			OldTruncated:     oldTruncated,
			NewTruncated:     newTruncated,
		}
	}
}

// FetchAllFileContent returns a command that fetches content for all files concurrently.
// Content is fetched with limits applied (max lines, max line length, max bytes).
func (m Model) FetchAllFileContent() tea.Cmd {
	if m.fetcher == nil {
		return nil
	}
	if len(m.files) == 0 {
		return nil
	}

	files := m.files
	fetcher := m.fetcher

	return func() tea.Msg {
		var wg sync.WaitGroup
		results := make([]FileContent, len(files))

		for i, fp := range files {
			wg.Add(1)
			go func(idx int, fp filePairInfo) {
				defer wg.Done()

				oldPath := stripPathPrefix(fp.oldPath)
				newPath := stripPathPrefix(fp.newPath)

				var oldContent, newContent []string
				var oldTruncated, newTruncated bool

				// Fetch old content (unless it's a new file)
				if fp.oldPath != "/dev/null" {
					lines, wasTruncated, err := fetcher.GetOldContentLines(oldPath)
					if err == nil {
						oldContent = lines
						oldTruncated = wasTruncated
					}
				}

				// Fetch new content (unless it's a deleted file)
				if fp.newPath != "/dev/null" {
					lines, wasTruncated, err := fetcher.GetNewContentLines(newPath)
					if err == nil {
						newContent = lines
						newTruncated = wasTruncated
					}
				}

				results[idx] = FileContent{
					FileIndex:        idx,
					OldContent:       oldContent,
					NewContent:       newContent,
					ContentTruncated: oldTruncated || newTruncated, // legacy field
					OldTruncated:     oldTruncated,
					NewTruncated:     newTruncated,
				}
			}(i, filePairInfo{oldPath: fp.OldPath, newPath: fp.NewPath})
		}

		wg.Wait()
		return AllContentLoadedMsg{Contents: results}
	}
}

// filePairInfo is a simple struct to pass file paths to goroutines.
type filePairInfo struct {
	oldPath string
	newPath string
}

// stripPathPrefix removes common git diff prefixes (a/, b/) from paths.
func stripPathPrefix(path string) string {
	if len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/") {
		return path[2:]
	}
	return path
}

// FetchSnapshotFilesContent returns a command that fetches content for files in a snapshot commit.
// It creates a fetcher using the snapshot's refs and fetches content for files in the given range.
// startIdx and endIdx define the file range in m.files (endIdx is exclusive).
func (m Model) FetchSnapshotFilesContent(oldRef, newRef string, startIdx, endIdx int) tea.Cmd {
	if m.git == nil || oldRef == "" || newRef == "" {
		return nil
	}
	if startIdx < 0 || endIdx <= startIdx || endIdx > len(m.files) {
		return nil
	}

	// Create a fetcher with the snapshot refs
	fetcher := content.NewFetcher(m.git, content.ModeDiffRefs, oldRef, newRef)

	// Capture the files to fetch
	filesToFetch := make([]filePairInfo, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		filesToFetch[i-startIdx] = filePairInfo{
			oldPath: m.files[i].OldPath,
			newPath: m.files[i].NewPath,
		}
	}

	return func() tea.Msg {
		var wg sync.WaitGroup
		results := make([]FileContent, len(filesToFetch))

		for i, fp := range filesToFetch {
			wg.Add(1)
			go func(idx int, fp filePairInfo, fileIdx int) {
				defer wg.Done()

				oldPath := stripPathPrefix(fp.oldPath)
				newPath := stripPathPrefix(fp.newPath)

				var oldContent, newContent []string
				var oldTruncated, newTruncated bool

				// Fetch old content (unless it's a new file)
				if fp.oldPath != "/dev/null" {
					lines, wasTruncated, err := fetcher.GetOldContentLines(oldPath)
					if err == nil {
						oldContent = lines
						oldTruncated = wasTruncated
					}
				}

				// Fetch new content (unless it's a deleted file)
				if fp.newPath != "/dev/null" {
					lines, wasTruncated, err := fetcher.GetNewContentLines(newPath)
					if err == nil {
						newContent = lines
						newTruncated = wasTruncated
					}
				}

				results[idx] = FileContent{
					FileIndex:        fileIdx, // Use actual file index in m.files
					OldContent:       oldContent,
					NewContent:       newContent,
					ContentTruncated: oldTruncated || newTruncated,
					OldTruncated:     oldTruncated,
					NewTruncated:     newTruncated,
				}
			}(i, fp, startIdx+i)
		}

		wg.Wait()
		return AllContentLoadedMsg{Contents: results}
	}
}
