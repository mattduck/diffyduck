// Package structure provides code structure extraction from tree-sitter parse trees.
// It extracts structural elements like functions, methods, and types to enable
// breadcrumb navigation showing the code hierarchy at a given position.
package structure

import (
	"regexp"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"
)

// Entry represents a structural element (function, type, etc.)
type Entry struct {
	StartLine  int      // 1-based line number
	EndLine    int      // 1-based line number (inclusive)
	Name       string   // e.g., "MyStruct", "myMethod"
	Kind       string   // e.g., "type", "func", "def", "class"
	Receiver   string   // Go only: e.g., "(m *Model)" - empty for functions and non-Go
	Params     []string // Function parameters: ["ctx context.Context", "request *Request"]
	ReturnType string   // Return type: "error", "User | None", etc. - empty if none
}

// FormatSignature formats the entry's signature for display, adapting to available width.
// Priority order: name > return type > params (filled in from left until space runs out).
// maxWidth of 0 means use compact format (no params shown).
func (e *Entry) FormatSignature(maxWidth int) string {
	// Types/classes have no signature
	if len(e.Params) == 0 && e.ReturnType == "" && e.Receiver == "" {
		return ""
	}

	// Build the name prefix (with receiver if present)
	name := e.Name
	if e.Receiver != "" {
		name = e.Receiver + " " + e.Name
	}

	// Compact format (maxWidth=0): name(...) -> ReturnType
	// Prioritizes return type, shows no params
	if maxWidth <= 0 {
		return e.formatWithParams(name, 0)
	}

	// Try progressively adding more params until it doesn't fit
	// Start with 0 params (compact), then add params one by one
	lastFit := e.formatWithParams(name, 0)

	for numParams := 1; numParams <= len(e.Params); numParams++ {
		sig := e.formatWithParams(name, numParams)
		if runewidth.StringWidth(sig) > maxWidth {
			// This doesn't fit, return the last one that did
			return lastFit
		}
		lastFit = sig
	}

	// All params fit
	return lastFit
}

// formatWithParams formats with the specified number of params, adding ... if truncated.
// numParams=0 shows (...) if there are params, or () if no params.
func (e *Entry) formatWithParams(name string, numParams int) string {
	var params string
	if len(e.Params) == 0 {
		params = "()"
	} else if numParams == 0 {
		params = "(...)"
	} else if numParams >= len(e.Params) {
		params = "(" + strings.Join(e.Params, ", ") + ")"
	} else {
		params = "(" + strings.Join(e.Params[:numParams], ", ") + ", ...)"
	}

	if e.ReturnType != "" {
		return name + params + " -> " + e.ReturnType
	}
	return name + params
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

// whitespaceRegex matches one or more whitespace characters (spaces, tabs, newlines).
var whitespaceRegex = regexp.MustCompile(`\s+`)

// bracketSpaceRegex matches spaces after opening brackets or before closing brackets.
var bracketSpaceRegex = regexp.MustCompile(`([(\[])\s+|\s+([)\]])`)

// normalizeWhitespace collapses all whitespace sequences (including newlines)
// into single spaces, and removes spaces adjacent to parentheses and brackets.
// This ensures multiline function signatures render properly in breadcrumbs.
func normalizeWhitespace(s string) string {
	// First collapse all whitespace to single spaces
	s = whitespaceRegex.ReplaceAllString(s, " ")
	// Then remove spaces after ([  and before ])
	s = bracketSpaceRegex.ReplaceAllString(s, "$1$2")
	return strings.TrimSpace(s)
}
