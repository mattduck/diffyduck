// Package structure provides code structure extraction from tree-sitter parse trees.
// It extracts structural elements like functions, methods, and types to enable
// breadcrumb navigation showing the code hierarchy at a given position.
package structure

import "sort"

// Entry represents a structural element (function, type, etc.)
type Entry struct {
	StartLine int    // 1-based line number
	EndLine   int    // 1-based line number (inclusive)
	Name      string // e.g., "MyStruct", "myMethod"
	Kind      string // e.g., "type", "func"
}

// Map holds sorted structure entries for fast lookup.
type Map struct {
	Entries []Entry // Sorted by StartLine
}

// AtLine returns entries that contain the given line number, ordered from
// outermost to innermost scope. Returns nil if no entries contain the line.
func (m *Map) AtLine(line int) []Entry {
	if m == nil || len(m.Entries) == 0 {
		return nil
	}

	// Binary search to find the first entry with StartLine > line.
	// Since entries are sorted by StartLine, we only need to check entries
	// before this index (entries after cannot contain our line).
	upperBound := sort.Search(len(m.Entries), func(i int) bool {
		return m.Entries[i].StartLine > line
	})

	// Collect entries that contain this line (only scan up to upperBound)
	var containing []Entry
	for i := 0; i < upperBound; i++ {
		e := m.Entries[i]
		if line <= e.EndLine {
			containing = append(containing, e)
		}
	}

	if len(containing) == 0 {
		return nil
	}

	// Sort by span size (largest first = outermost first)
	sort.Slice(containing, func(i, j int) bool {
		spanI := containing[i].EndLine - containing[i].StartLine
		spanJ := containing[j].EndLine - containing[j].StartLine
		return spanI > spanJ
	})

	return containing
}

// NewMap creates a Map from a slice of entries, sorting them by StartLine.
func NewMap(entries []Entry) *Map {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartLine < sorted[j].StartLine
	})
	return &Map{Entries: sorted}
}
