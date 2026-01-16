package tui

import (
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// FetchFileContent returns a command that fetches content for one file.
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

		// Fetch old content (unless it's a new file)
		if fp.OldPath != "/dev/null" {
			content, err := fetcher.GetOldContent(oldPath)
			if err == nil && content != "" {
				oldContent = strings.Split(content, "\n")
			}
		}

		// Fetch new content (unless it's a deleted file)
		if fp.NewPath != "/dev/null" {
			content, err := fetcher.GetNewContent(newPath)
			if err == nil && content != "" {
				newContent = strings.Split(content, "\n")
			}
		}

		return FileContentLoadedMsg{
			FileIndex:  fileIndex,
			OldContent: oldContent,
			NewContent: newContent,
		}
	}
}

// FetchAllFileContent returns a command that fetches content for all files concurrently.
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

				// Fetch old content (unless it's a new file)
				if fp.oldPath != "/dev/null" {
					content, err := fetcher.GetOldContent(oldPath)
					if err == nil && content != "" {
						oldContent = strings.Split(content, "\n")
					}
				}

				// Fetch new content (unless it's a deleted file)
				if fp.newPath != "/dev/null" {
					content, err := fetcher.GetNewContent(newPath)
					if err == nil && content != "" {
						newContent = strings.Split(content, "\n")
					}
				}

				results[idx] = FileContent{
					FileIndex:  idx,
					OldContent: oldContent,
					NewContent: newContent,
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
