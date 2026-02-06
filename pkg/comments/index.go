package comments

import (
	"slices"
	"sort"
	"strings"
)

// Index maps file paths to comment IDs for efficient lookup.
type Index struct {
	// entries maps file path to list of comment IDs.
	entries map[string][]string
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		entries: make(map[string][]string),
	}
}

// Add adds a comment ID to the index for a file.
func (idx *Index) Add(filePath, commentID string) {
	ids := idx.entries[filePath]
	if slices.Contains(ids, commentID) {
		return
	}
	idx.entries[filePath] = append(ids, commentID)
}

// Remove removes a comment ID from the index.
func (idx *Index) Remove(filePath, commentID string) {
	ids := idx.entries[filePath]
	i := slices.Index(ids, commentID)
	if i == -1 {
		return
	}
	idx.entries[filePath] = slices.Delete(ids, i, i+1)
	// Clean up empty entries
	if len(idx.entries[filePath]) == 0 {
		delete(idx.entries, filePath)
	}
}

// Get returns all comment IDs for a file.
func (idx *Index) Get(filePath string) []string {
	return idx.entries[filePath]
}

// Files returns all file paths that have comments.
func (idx *Index) Files() []string {
	files := make([]string, 0, len(idx.entries))
	for f := range idx.entries {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// All returns all comment IDs in the index.
func (idx *Index) All() []string {
	var all []string
	for _, ids := range idx.entries {
		all = append(all, ids...)
	}
	sort.Strings(all)
	return all
}

// Serialize converts the index to its storage format.
// Format: one line per entry as "file:<path>:<commentID>"
func (idx *Index) Serialize() string {
	var lines []string
	for filePath, ids := range idx.entries {
		for _, id := range ids {
			lines = append(lines, "file:"+filePath+":"+id)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// ParseIndex parses the index from its storage format.
func ParseIndex(data string) *Index {
	idx := NewIndex()
	if data == "" {
		return idx
	}

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "file:") {
			continue
		}

		// Parse "file:<path>:<id>"
		rest := strings.TrimPrefix(line, "file:")
		lastColon := strings.LastIndex(rest, ":")
		if lastColon == -1 {
			continue
		}
		filePath := rest[:lastColon]
		commentID := rest[lastColon+1:]
		if filePath != "" && commentID != "" {
			idx.Add(filePath, commentID)
		}
	}

	return idx
}
