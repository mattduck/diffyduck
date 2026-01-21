package tui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// FetchFileContent returns a command that fetches content for one file.
// Content is fetched with limits applied (max lines, max line length, max bytes).
func (m Model) FetchFileContent(fileIndex int) tea.Cmd {
	if m.fetcher == nil {
		return nil
	}
	if fileIndex < 0 || fileIndex >= len(m.files) {
		return nil
	}

	fp := m.files[fileIndex]
	fetcher := m.fetcher

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
