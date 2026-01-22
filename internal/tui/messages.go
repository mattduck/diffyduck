package tui

// FileContentLoadedMsg is sent when file content has been fetched for a single file.
type FileContentLoadedMsg struct {
	FileIndex        int
	OldContent       []string
	NewContent       []string
	ContentTruncated bool // true if content was truncated due to limits (legacy, use per-side)
	OldTruncated     bool // true if old content was truncated
	NewTruncated     bool // true if new content was truncated
	Err              error
}

// AllContentLoadedMsg is sent when content for all files has been fetched.
type AllContentLoadedMsg struct {
	Contents []FileContent
}

// FileContent holds the fetched content for a single file.
type FileContent struct {
	FileIndex        int
	OldContent       []string
	NewContent       []string
	ContentTruncated bool // true if content was truncated due to limits (legacy, use per-side)
	OldTruncated     bool // true if old content was truncated
	NewTruncated     bool // true if new content was truncated
	Err              error
}

// HighlightReadyMsg is sent when syntax highlighting spans are ready for a file.
type HighlightReadyMsg struct {
	FileIndex    int
	OldSpans     []HighlightSpan  // spans for old file content
	NewSpans     []HighlightSpan  // spans for new file content
	NewStructure []StructureEntry // structure for new file content (for breadcrumbs)
}

// HighlightSpan represents a highlighted range with a category.
// This is a copy of highlight.Span to avoid import cycles.
type HighlightSpan struct {
	Start    int // byte offset
	End      int // byte offset (exclusive)
	Category int // highlight.Category value
}

// PairsHighlightReadyMsg is sent when syntax highlighting from Pairs is ready for a file.
type PairsHighlightReadyMsg struct {
	FileIndex     int
	OldSpans      []HighlightSpan // spans for concatenated old lines
	NewSpans      []HighlightSpan // spans for concatenated new lines
	OldLineStarts map[int]int     // line number -> byte offset
	NewLineStarts map[int]int     // line number -> byte offset
	OldLineLens   map[int]int     // line number -> line length
	NewLineLens   map[int]int     // line number -> line length
}

// StructureEntry represents a structural code element for breadcrumbs.
// This mirrors structure.Entry to avoid import in messages.
type StructureEntry struct {
	StartLine int    // 1-based line number
	EndLine   int    // 1-based line number (inclusive)
	Name      string // e.g., "MyStruct", "myMethod"
	Kind      string // e.g., "type", "func"
	Signature string // e.g., "(m Model) myMethod(ctx)" - includes receiver and params
}
